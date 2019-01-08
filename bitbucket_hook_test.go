package git

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBitbucketDeployPush(t *testing.T) {
	repo := &Repo{Branch: "master", Hook: HookConfig{URL: "/bitbucket_deploy"}}
	bbHook := BitbucketHook{}

	for i, test := range []struct {
		ip           string
		body         string
		event        string
		responseBody string
		code         int
	}{
		{"104.192.136.20", "", "", "", 400},
		{"131.103.20.160", "", "", "", 403},
		{"104.192.136.120", "", "repo:push", "", 400},
		{"34.198.203.127", pushBBBodyValid, "repo:push", "", 200},
		{"34.198.178.64", pushBBBodyValid, "repo:push", "", 200},
		{"34.198.32.85", pushBBBodyEmptyBranch, "repo:push", "", 400},
		{"104.192.143.2", pushBBBodyDeleteBranch, "repo:push", "", 400},
	} {

		req, err := http.NewRequest("POST", "/bitbucket_deploy", bytes.NewBuffer([]byte(test.body)))
		if err != nil {
			t.Fatalf("Test %v: Could not create HTTP request: %v", i, err)
		}
		req.RemoteAddr = test.ip

		if test.event != "" {
			req.Header.Add("X-Event-Key", test.event)
		}

		rec := httptest.NewRecorder()

		code, err := bbHook.Handle(rec, req, repo)

		if code != test.code {
			t.Errorf("Test %d: Expected response code to be %d but was %d", i, test.code, code)
		}

		if rec.Body.String() != test.responseBody {
			t.Errorf("Test %d: Expected response body to be '%v' but was '%v'", i, test.responseBody, rec.Body.String())
		}
	}

}

var pushBBBodyEmptyBranch = `
{
  "push": {
    "changes": [
      {
        "new": {
          "type": "branch",
          "name": "",
          "target": {
            "hash": "709d658dc5b6d6afcd46049c2f332ee3f515a67d"
          }
        }
      }
    ]
  }
}
`

var pushBBBodyValid = `
{
  "push": {
    "changes": [
      {
        "new": {
          "type": "branch",
          "name": "master",
          "target": {
            "hash": "709d658dc5b6d6afcd46049c2f332ee3f515a67d"
          }
        }
      }
    ]
  }
}
`

var pushBBBodyDeleteBranch = `
{
  "push": {
    "changes": [
    ]
  }
}
`
