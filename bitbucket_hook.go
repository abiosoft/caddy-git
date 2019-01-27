package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BitbucketHook is webhook for BitBucket.org.
type BitbucketHook struct{}

type bbPush struct {
	Push struct {
		Changes []struct {
			New struct {
				Name string `json:"name,omitempty"`
			} `json:"new,omitempty"`
		} `json:"changes,omitempty"`
	} `json:"push,omitempty"`
}

// DoesHandle satisfies hookHandler.
func (b BitbucketHook) DoesHandle(h http.Header) bool {
	event := h.Get("X-Event-Key")

	// for Gitlab you can only use X-Gitlab-Event header to test if you could handle the request
	if event != "" {
		return true
	}
	return false
}

// Handle satisfies hookHandler.
func (b BitbucketHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if !b.verifyBitbucketIP(r.RemoteAddr) {
		return http.StatusForbidden, errors.New("the request doesn't come from a valid IP")
	}

	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestTimeout, errors.New("could not read body from request")
	}

	event := r.Header.Get("X-Event-Key")
	if event == "" {
		return http.StatusBadRequest, errors.New("the 'X-Event-Key' header is required but was missing")
	}

	switch event {
	case "repo:push":
		err = b.handlePush(body, repo)
		if !hookIgnored(err) && err != nil {
			return http.StatusBadRequest, err
		}
	default:
		// return 400 if we do not handle the event type.
		return http.StatusBadRequest, nil
	}

	return http.StatusOK, err
}

func (b BitbucketHook) handlePush(body []byte, repo *Repo) error {
	var push bbPush

	err := json.Unmarshal(body, &push)
	if err != nil {
		return err
	}

	if len(push.Push.Changes) == 0 {
		return errors.New("the push was incomplete, missing change list")
	}

	change := push.Push.Changes[0]
	if len(change.New.Name) == 0 {
		return errors.New("the push didn't contain a valid branch name")
	}

	branch := change.New.Name
	if branch != repo.Branch {
		return hookIgnoredError{hookType: hookName(b), err: fmt.Errorf("found different branch %v", branch)}
	}
	Logger().Print("Received pull notification for the tracking branch, updating...\n")
	repo.Pull()

	return nil
}

func cleanRemoteIP(remoteIP string) string {
	// *httpRequest.RemoteAddr comes in format IP:PORT, remove the port
	return strings.Split(remoteIP, ":")[0]
}

func (b BitbucketHook) verifyBitbucketIP(remoteIP string) bool {
	ipAddress := net.ParseIP(cleanRemoteIP(remoteIP))

	updateBitBucketIPs()

	atlassianIPsMu.Lock()
	ipItems := atlassianIPs.Items
	atlassianIPsMu.Unlock()

	// if there was a problem getting list of IPs, might as well
	// allow it since it could still need authentication anyway
	if len(ipItems) == 0 {
		return true
	}

	for _, item := range ipItems {
		// it may be regular ip address
		if !strings.Contains(item.CIDR, "/") {
			ip := net.ParseIP(item.CIDR)
			if ip.Equal(ipAddress) {
				return true
			}
			continue
		}

		_, cidrnet, err := net.ParseCIDR(item.CIDR)
		if err != nil {
			Logger().Printf("Error parsing CIDR block [%s]. Skipping...\n", item.CIDR)
			continue
		}

		if cidrnet.Contains(ipAddress) {
			return true
		}
	}

	return false
}

func updateBitBucketIPs() {
	atlassianIPsMu.Lock()
	defer atlassianIPsMu.Unlock()

	// if list of IP ranges is outdated, get latest
	if atlassianIPs.lastUpdated.IsZero() ||
		time.Since(atlassianIPs.lastUpdated) > 24*time.Hour {

		resp, err := http.Get("https://ip-ranges.atlassian.com/")
		if err != nil {
			// allow, since we can't know for sure either way, I guess
			Logger().Printf("[ERROR] Requesting recent IPs for bitbucket: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			Logger().Printf("[ERROR] Getting recent IPs for bitbucket: HTTP %d", resp.StatusCode)
			return
		}

		var newIPs atlassianIPResponse
		err = json.NewDecoder(resp.Body).Decode(&newIPs)
		if err != nil {
			Logger().Printf("[ERROR] Decoding recent IPs for bitbucket: %v", err)
			return
		}

		// replace the IP list
		newIPs.lastUpdated = time.Now()
		atlassianIPs = newIPs
	}
}

type atlassianIPResponse struct {
	CreationDate string             `json:"creationDate"`
	SyncToken    int                `json:"syncToken"`
	Items        []atlassianIPRange `json:"items"`

	lastUpdated time.Time // added by us
}

type atlassianIPRange struct {
	Network string `json:"network"`
	MaskLen int    `json:"mask_len"`
	CIDR    string `json:"cidr"`
	Mask    string `json:"mask"`
}

var (
	atlassianIPs   atlassianIPResponse
	atlassianIPsMu sync.Mutex
)
