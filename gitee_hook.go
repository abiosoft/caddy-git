package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// GiteeHook is webhook for gitee.com
type GiteeHook struct{}

type giteePush struct {
	Ref string `json:"ref"`
}

// DoesHandle satisfies hookHandler.
func (g GiteeHook) DoesHandle(h http.Header) bool {
	event := h.Get("X-Gitee-Event")

	// for Gitee you can use X-Gitee-Event header to test if you could handle the request
	if event != "" {
		return true
	}
	return false
}

// Handle satisfies hookHandler.
func (g GiteeHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestTimeout, errors.New("could not read body from request")
	}

	err = g.handleToken(r, body, repo.Hook.Secret)
	if err != nil {
		return http.StatusBadRequest, err
	}

	event := r.Header.Get("X-Gitee-Event")
	if event == "" {
		return http.StatusBadRequest, errors.New("the 'X-Gitee-Event' header is required but was missing")
	}

	switch event {
	case "Push Hook":
		err = g.handlePush(body, repo)
		if !hookIgnored(err) && err != nil {
			return http.StatusBadRequest, err
		}

		// return 400 if we do not handle the event type.
	default:
		return http.StatusBadRequest, nil
	}

	return http.StatusOK, err
}

// handleToken checks for an optional token in the request. Gitee's webhook tokens are just
// simple strings that get sent as a header with the hook request. If one
// exists, verify that it matches the secret in the Caddy configuration.
func (g GiteeHook) handleToken(r *http.Request, body []byte, secret string) error {
	token := r.Header.Get("X-Gitee-Token")
	if token != "" {
		if secret == "" {
			Logger().Print("Unable to verify request. Secret not set in caddyfile!\n")
		} else {
			if token != secret {
				return errors.New("Unable to verify request. The token and specified secret do not match!")
			}
		}
	}

	return nil
}

func (g GiteeHook) handlePush(body []byte, repo *Repo) error {
	var push giteePush

	err := json.Unmarshal(body, &push)
	if err != nil {
		return err
	}

	// extract the branch being pushed from the ref string
	// and if it matches with our locally tracked one, pull.
	refSlice := strings.Split(push.Ref, "/")
	if len(refSlice) != 3 {
		return errors.New("the push request contained an invalid reference string")
	}

	branch := refSlice[2]
	if branch != repo.Branch {
		return hookIgnoredError{hookType: hookName(g), err: fmt.Errorf("found different branch %v", branch)}
	}

	Logger().Print("Received pull notification for the tracking branch, updating...\n")
	repo.Pull()

	return nil
}
