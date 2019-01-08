package git

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiteeDeployPush(t *testing.T) {
	repo := &Repo{Branch: "master", Hook: HookConfig{URL: "/gitee_deploy"}}
	glHook := GiteeHook{}

	for i, test := range []struct {
		body         string
		event        string
		responseBody string
		code         int
	}{
		{"", "", "", 400},
		{"", "Push Hook", "", 400},
		{pushGiteeBodyOther, "Push Hook", "", 200},
		{pushGiteeBodyPartial, "Push Hook", "", 400},
		{"", "Some other Event", "", 400},
	} {

		req, err := http.NewRequest("POST", "/gitee_deploy", bytes.NewBuffer([]byte(test.body)))
		if err != nil {
			t.Fatalf("Test %v: Could not create HTTP request: %v", i, err)
		}

		if test.event != "" {
			req.Header.Add("X-Gitee-Event", test.event)
		}

		rec := httptest.NewRecorder()

		code, err := glHook.Handle(rec, req, repo)

		if code != test.code {
			t.Errorf("Test %d: Expected response code to be %d but was %d", i, test.code, code)
		}

		if rec.Body.String() != test.responseBody {
			t.Errorf("Test %d: Expected response body to be '%v' but was '%v'", i, test.responseBody, rec.Body.String())
		}
	}

}

var pushGiteeBodyPartial = `
{
  "ref": ""
}
`

var pushGiteeBodyOther = `
{
  "ref": "refs/heads/some-other-branch"
}
`
