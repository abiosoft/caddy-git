package git

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
)

type GenericHook struct{}


type gPush struct {
	Ref string `json:"ref"`
}

func (g GenericHook) DoesHandle(h http.Header) bool {
	return true
}

func (g GenericHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method.")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestTimeout, errors.New("could not read body from request")
	}

	err = g.handlePush(body, repo)
	if err != nil {
		return http.StatusBadRequest, err
	}


	return http.StatusOK, nil
}


func (g GenericHook) handlePush(body []byte, repo *Repo) error {
	var push gPush

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
