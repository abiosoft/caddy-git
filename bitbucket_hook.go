package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

// See: https://confluence.atlassian.com/bitbucket/manage-webhooks-735643732.html
var bitbucketIPBlocks = []string{
	"104.192.136.0/21",
	"34.198.203.127",
	"34.198.178.64",
	"34.198.32.85",
}

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
	for _, cidr := range bitbucketIPBlocks {
		// it may be regular ip address
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip.Equal(ipAddress) {
				return true
			}
			continue
		}

		_, cidrnet, err := net.ParseCIDR(cidr)
		if err != nil {
			Logger().Printf("Error parsing CIDR block [%s]. Skipping...\n", cidr)
			continue
		}

		if cidrnet.Contains(ipAddress) {
			return true
		}
	}
	return false
}
