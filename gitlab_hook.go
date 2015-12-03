package git

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
)

type GitlabHook struct{}

type glPush struct {
	Ref string `json:"ref"`
}

func (g GitlabHook) DoesHandle(h http.Header) bool {
	event := h.Get("X-Gitlab-Event")

	// for Gitlab you can only use X-Gitlab-Event header to test if you could handle the request
	if event != "" {
		return true
	}
	return false
}

func (g GitlabHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method.")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestTimeout, errors.New("could not read body from request")
	}

	event := r.Header.Get("X-Gitlab-Event")
	if event == "" {
		return http.StatusBadRequest, errors.New("the 'X-Gitlab-Event' header is required but was missing.")
	}

	switch event {
	case "Push Hook":
		err := g.handlePush(body, repo)
		if err != nil {
			return http.StatusBadRequest, err
		}

	// return 400 if we do not handle the event type.
	default:
		return http.StatusBadRequest, nil
	}

	return http.StatusOK, nil
}

func (g GitlabHook) handlePush(body []byte, repo *Repo) error {
	var push glPush

	err := json.Unmarshal(body, &push)
	if err != nil {
		return err
	}

	// extract the branch being pushed from the ref string
	// and if it matches with our locally tracked one, pull.
	refSlice := strings.Split(push.Ref, "/")
	if len(refSlice) != 3 {
		return errors.New("the push request contained an invalid reference string.")
	}

	branch := refSlice[2]
	if branch == repo.Branch {
		Logger().Print("Received pull notification for the tracking branch, updating...\n")
		repo.Pull()
	}

	return nil
}
