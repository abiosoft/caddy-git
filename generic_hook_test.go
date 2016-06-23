package git

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenericDeployPush(t *testing.T) {
	repo := &Repo{Branch: "master", Hook: HookConfig{URL: "/generic_deploy"}}
	gHook := GenericHook{}

	for i, test := range []struct {
		body         string
		event        string
		responseBody string
		code         int
	}{
		{"", "", "", 400},
		{pushGBodyOther, "", "", 200},
		{pushGBodyPartial, "", "", 400},
		{"", "Some Event", "", 400},
	} {

		req, err := http.NewRequest("POST", "/generic_deploy", bytes.NewBuffer([]byte(test.body)))
		if err != nil {
			t.Fatalf("Test %v: Could not create HTTP request: %v", i, err)
		}

		rec := httptest.NewRecorder()

		code, err := gHook.Handle(rec, req, repo)

		if code != test.code {
			t.Errorf("Test %d: Expected response code to be %d but was %d", i, test.code, code)
		}

		if rec.Body.String() != test.responseBody {
			t.Errorf("Test %d: Expected response body to be '%v' but was '%v'", i, test.responseBody, rec.Body.String())
		}
	}

}

var pushGBodyPartial = `
{
  "ref": ""
}
`

var pushGBodyOther = `
{
  "ref": "refs/heads/some-other-branch"
}
`
