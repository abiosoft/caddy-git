package git

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGogsDeployPush(t *testing.T) {
	repo := &Repo{Branch: "master", Hook: HookConfig{URL: "/gogs_deploy"}}
	gsHook := GogsHook{}

	for i, test := range []struct {
		body         string
		event        string
		responseBody string
		code         int
	}{
		{"", "", "", 400},
		{"", "push", "", 400},
		{pushGSBodyOther, "push", "", 200},
		{pushGSBodyPartial, "push", "", 400},
		{"", "ping", "pong", 200},
	} {

		req, err := http.NewRequest("POST", "/gogs_deploy", bytes.NewBuffer([]byte(test.body)))
		if err != nil {
			t.Fatalf("Test %v: Could not create HTTP request: %v", i, err)
		}

		if test.event != "" {
			req.Header.Add("X-Gogs-Event", test.event)
		}

		rec := httptest.NewRecorder()

		code, err := gsHook.Handle(rec, req, repo)

		if code != test.code {
			t.Errorf("Test %d: Expected response code to be %d but was %d", i, test.code, code)
		}

		if rec.Body.String() != test.responseBody {
			t.Errorf("Test %d: Expected response body to be '%v' but was '%v'", i, test.responseBody, rec.Body.String())
		}
	}

}

var pushGSBodyPartial = `
{
  "ref": ""
}
`

var pushGSBodyOther = `
{
  "ref": "refs/heads/some-other-branch"
}
`
