package git

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/abiosoft/caddy-git/gittest"
	"github.com/mholt/caddy"
)

// init sets the OS used to fakeOS
func init() {
	SetOS(gittest.FakeOS)
}

func TestGitSetup(t *testing.T) {
	c := caddy.NewTestController("http", `git ssh://git@github.com:mholt/caddy.git`)
	err := setup(c)
	check(t, err)
}

func TestGitParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  *Repo
	}{
		{`git git@github.com:user/repo {
			args --depth 1
			key ~/.key
		}`, false, &Repo{
			URL:       "ssh://git@github.com:user/repo.git",
			CloneArgs: []string{"--depth", "1"},
		}},
		{`git user:pass@github.com/user/repo {
			args --depth 1
		}`, false, &Repo{
			URL:       "https://user:pass@github.com/user/repo.git",
			CloneArgs: []string{"--depth", "1"},
		}},
		{`git {
		repo ssh://git@github.com:user/repo
		key ~/.key
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@github.com:user/repo.git",
		}},
		{`git {
		repo ssh://git@github.com:user/repo
		key ~/.key
		interval 600
		}`, false, &Repo{
			KeyPath:  "~/.key",
			URL:      "ssh://git@github.com:user/repo.git",
			Interval: time.Second * 600,
		}},
		{`git {
		key ~/.key
		}`, true, nil},
		{`git {
		repo ssh://git@github.com:user/repo
		key ~/.key
		then echo hello world
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@github.com:user/repo.git",
			Then:    []Then{NewThen("echo", "hello world")},
		}},
		{`git https://user@bitbucket.org/user/repo.git`, false, &Repo{
			URL: "https://user@bitbucket.org/user/repo.git",
		}},
		{`git ssh://git@bitbucket.org:user/repo.git {
			key ~/.key
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:user/repo.git",
		}},
		{`git ssh://git@bitbucket.org:user/repo.git {
			key ~/.key
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:user/repo.git",
		}},
		{`git ssh://git@bitbucket.org:2222/user/repo.git {
			key ~/.key
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:2222/user/repo.git",
		}},
		{`git ssh://git@bitbucket.org:2222/user/repo.git {
			key ~/.key
			hook_type gogs
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:2222/user/repo.git",
			Hook: HookConfig{
				Type: "gogs",
			},
		}},
		{`git ssh://git@bitbucket.org:2222/user/repo.git {
			key ~/.key
			hook /webhook
			hook_type gogs
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:2222/user/repo.git",
			Hook: HookConfig{
				URL:  "/webhook",
				Type: "gogs",
			},
		}},
		{`git ssh://git@bitbucket.org:2222/user/repo.git {
			key ~/.key
			hook /webhook some-secrets
			hook_type gogs
		}`, false, &Repo{
			KeyPath: "~/.key",
			URL:     "ssh://git@bitbucket.org:2222/user/repo.git",
			Hook: HookConfig{
				URL:    "/webhook",
				Secret: "some-secrets",
				Type:   "gogs",
			},
		}},
	}

	for i, test := range tests {
		c := caddy.NewTestController("http", test.input)
		git, err := parse(c)
		if !test.shouldErr && err != nil {
			t.Errorf("Test %v should not error but found %v", i, err)
			continue
		}
		if test.shouldErr && err == nil {
			t.Errorf("Test %v should error but found nil", i)
			continue
		}
		repo := git.Repo(0)
		if !reposEqual(test.expected, repo) {
			t.Errorf("Test %v expects %v but found %v", i, test.expected, repo)
		}
	}
}

func TestIntervals(t *testing.T) {
	tests := []string{
		`git user:pass@github.com/user/repo { interval 10 }`,
		`git user:pass@github.com/user/repo { interval 1 }`,
	}

	for i, test := range tests {
		SetLogger(gittest.NewLogger(gittest.Open("file")))
		c1 := caddy.NewTestController("http", test)
		git, err := parse(c1)
		check(t, err)
		repo := git.Repo(0)

		c2 := caddy.NewTestController("http", test)
		err = setup(c2)
		check(t, err)

		// start startup services
		err = func() error {
			// Start service routine in background
			Start(repo)
			// Do a pull right away to return error
			return repo.Pull()
		}()
		check(t, err)

		// wait for first background pull
		gittest.Sleep(time.Millisecond * 100)

		// switch logger to test file
		logFile := gittest.Open("file")
		SetLogger(gittest.NewLogger(logFile))

		// sleep for the interval
		gittest.Sleep(repo.Interval)

		// get log output
		out, err := ioutil.ReadAll(logFile)
		check(t, err)

		// if greater than minimum interval
		if repo.Interval >= time.Second*5 {
			expected := `https://user@github.com/user/repo.git pulled.
No new changes.`

			// ensure pull is done by tracing the output
			if expected != strings.TrimSpace(string(out)) {
				t.Errorf("Test %v: Expected %v found %v", i, expected, string(out))
			}
		} else {
			// ensure pull is ignored by confirming no output
			if string(out) != "" {
				t.Errorf("Test %v: Expected no output but found %v", i, string(out))
			}
		}

		// stop background thread monitor
		Services.Stop(string(repo.URL), 1)

	}

}

func reposEqual(expected, repo *Repo) bool {
	thenStr := func(then []Then) string {
		var str []string
		for _, t := range then {
			str = append(str, t.Command())
		}
		return fmt.Sprint(str)
	}
	if expected == nil {
		return repo == nil
	}
	if expected.Branch != "" && expected.Branch != repo.Branch {
		return false
	}
	if expected.Host != "" && expected.Host != repo.Host {
		return false
	}
	if expected.Interval != 0 && expected.Interval != repo.Interval {
		return false
	}
	if expected.KeyPath != "" && expected.KeyPath != repo.KeyPath {
		return false
	}
	if expected.Path != "" && expected.Path != repo.Path {
		return false
	}
	if expected.Then != nil && thenStr(expected.Then) != thenStr(repo.Then) {
		return false
	}
	if expected.URL != "" && expected.URL != repo.URL {
		return false
	}
	if fmt.Sprint(expected.Hook) != fmt.Sprint(repo.Hook) {
		return false
	}
	if fmt.Sprint(expected.CloneArgs) != fmt.Sprint(repo.CloneArgs) {
		return false
	}
	return true
}
