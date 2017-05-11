package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// GogsHook is the webhook for gogs.io.
type GogsHook struct{}

type gsPush struct {
	Ref string `json:"ref"`
}

// DoesHandle satisfies hookHandler.
func (g GogsHook) DoesHandle(h http.Header) bool {
	event := h.Get("X-Gogs-Event")

	// for Gogs you can only use X-Gogs-Event header to test if you could handle the request
	if event != "" {
		return true
	}
	return false
}

// Handle satisfies hookHandler.
func (g GogsHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method")
	}

	// read full body - required for signature
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		return http.StatusBadRequest, err
	}

	event := r.Header.Get("X-Gogs-Event")
	if event == "" {
		return http.StatusBadRequest, errors.New("the 'X-Gogs-Event' header is required but was missing")
	}

	switch event {
	case "ping":
		w.Write([]byte("pong"))
	case "push":
		err = g.handlePush(body, repo)
		if !hookIgnored(err) && err != nil {
			return http.StatusBadRequest, err
		}

	// return 400 if we do not handle the event type.
	// This is to visually show the user a configuration error in the Gogs ui.
	default:
		return http.StatusBadRequest, nil
	}

	return http.StatusOK, err
}

func (g GogsHook) handlePush(body []byte, repo *Repo) error {
	var push gsPush

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
