package git

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/abiosoft/caddy-git/testutils"
	"reflect"
)

// init sets the OS used to fakeOS.
func init() {
	SetOS(testutils.FakeOS)
}

func check(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Error not expected but found %v", err)
	}
}

func TestInit(t *testing.T) {
	err := Init()
	check(t, err)
}

func TestHelpers(t *testing.T) {
	Init()
	f, err := writeScriptFile([]byte("script"))
	check(t, err)
	var b [6]byte
	_, err = f.Read(b[:])
	check(t, err)
	if string(b[:]) != "script" {
		t.Errorf("Expected script found %v", string(b[:]))
	}

	out, err := runCmdOutput(gitBinary, []string{"-version"}, "")
	check(t, err)
	if out != testutils.CmdOutput {
		t.Errorf("Expected %v found %v", testutils.CmdOutput, out)
	}

	err = runCmd(gitBinary, []string{"-version"}, "")
	check(t, err)

	wScript := gitWrapperScript()
	if string(wScript) != expectedWrapperScript {
		t.Errorf("Expected %v found %v", expectedWrapperScript, string(wScript))
	}

	f, err = writeScriptFile(wScript)
	check(t, err)

	repo := &Repo{Host: "github.com", KeyPath: "~/.key"}
	script := string(bashScript(f.Name(), repo, []string{"clone", "git@github.com/repo/user"}))
	if script != expectedBashScript {
		t.Errorf("Expected %v found %v", expectedBashScript, script)
	}
}

func TestGit(t *testing.T) {
	// prepare
	repos := []*Repo{
		nil,
		&Repo{Path: "gitdir", URL: "success.git"},
	}
	for _, r := range repos {
		repo := createRepo(r)
		err := repo.Prepare()
		check(t, err)
	}

	// pull with success
	logFile := testutils.Open("file")
	SetLogger(log.New(logFile, "", 0))
	tests := []struct {
		repo   *Repo
		output string
	}{
		{
			&Repo{Path: "gitdir", URL: "git@github.com:user/repo.git", KeyPath: "~/.key", Then: []Then{NewThen("echo", "Hello")}},
			`git@github.com:user/repo.git pulled.
Command 'echo Hello' successful.
`,
		},
		{
			&Repo{Path: "gitdir", URL: "https://github.com/user/repo.git", Then: []Then{NewThen("echo", "Hello")}},
			`https://github.com/user/repo.git pulled.
Command 'echo Hello' successful.
`,
		},
		{
			&Repo{URL: "git@github.com:user/repo"},
			`git@github.com:user/repo pulled.
`,
		},
	}

	for i, test := range tests {
		testutils.CmdOutput = test.repo.URL

		test.repo = createRepo(test.repo)

		statusChangeHandler := &testStatusChangeHandlerImpl{}
		err := test.repo.registerOnStatusChange(statusChangeHandler)
		check(t, err)

		err = test.repo.Prepare()
		check(t, err)

		err = test.repo.Pull()
		check(t, err)

		out, err := ioutil.ReadAll(logFile)
		check(t, err)
		if test.output != string(out) {
			t.Errorf("Pull with Success %v: Expected %v found %v", i, test.output, string(out))
		}

		if statusChangeHandler.numberOfCalls != 1 {
			t.Errorf("Number of status change handler calls. Expected %v found %v", 1, statusChangeHandler.numberOfCalls)
		}
		if statusChangeHandler.lastRepo != test.repo {
			t.Errorf("Repo provided to status change handler. Expected %v found %v", test.repo, statusChangeHandler.lastRepo)
		}
		if !reflect.DeepEqual(statusChangeHandler.lastStatus, test.repo.toStatus()) {
			t.Errorf("Status provided to status change handler. Expected %v found %v", test.repo.toStatus(), statusChangeHandler.lastStatus)
		}
	}

	// pull with error
	repos = []*Repo{
		&Repo{Path: "gitdir", URL: "http://github.com:u/repo.git"},
		&Repo{Path: "gitdir", URL: "https://github.com/user/repo.git", Then: []Then{NewThen("echo", "Hello")}},
		&Repo{Path: "gitdir"},
		&Repo{Path: "gitdir", KeyPath: ".key"},
	}

	testutils.CmdOutput = "git@github.com:u1/repo.git"
	for i, repo := range repos {
		repo = createRepo(repo)

		err := repo.Prepare()
		if err == nil {
			t.Errorf("Pull with Error %v: Error expected but not found %v", i, err)
			continue
		}

		expected := "another git repo 'git@github.com:u1/repo.git' exists at gitdir"
		if expected != err.Error() {
			t.Errorf("Pull with Error %v: Expected %v found %v", i, expected, err.Error())
		}
	}

	// timeout checks
	timeoutTests := []struct {
		repo       *Repo
		shouldPull bool
	}{
		{&Repo{Interval: time.Millisecond * 4900}, false},
		{&Repo{Interval: time.Millisecond * 1}, false},
		{&Repo{Interval: time.Second * 5}, true},
		{&Repo{Interval: time.Second * 10}, true},
	}

	for i, r := range timeoutTests {
		r.repo = createRepo(r.repo)

		err := r.repo.Prepare()
		check(t, err)
		err = r.repo.Pull()
		check(t, err)

		before := r.repo.lastPull

		testutils.Sleep(r.repo.Interval)

		err = r.repo.Pull()
		after := r.repo.lastPull
		check(t, err)

		expected := after.After(before)
		if expected != r.shouldPull {
			t.Errorf("Pull with Error %v: Expected %v found %v", i, expected, r.shouldPull)
		}
	}

}

func createRepo(r *Repo) *Repo {
	repo := &Repo{
		URL:      "git@github.com/user/test",
		Path:     ".",
		Host:     "github.com",
		Branch:   "master",
		Interval: time.Second * 60,
		seState: newStatusEndpointState(),
	}
	if r == nil {
		return repo
	}
	if r.Branch != "" {
		repo.Branch = r.Branch
	}
	if r.Host != "" {
		repo.Branch = r.Branch
	}
	if r.Interval != 0 {
		repo.Interval = r.Interval
	}
	if r.KeyPath != "" {
		repo.KeyPath = r.KeyPath
	}
	if r.Path != "" {
		repo.Path = r.Path
	}
	if r.Then != nil {
		repo.Then = r.Then
	}
	if r.URL != "" {
		repo.URL = r.URL
	}

	return repo
}

type testStatusChangeHandlerImpl struct {
	lastRepo      *Repo
	lastStatus    Status
	numberOfCalls int
}

func (handler *testStatusChangeHandlerImpl) handle(repo *Repo, status Status) bool {
	handler.lastRepo = repo
	handler.lastStatus = status
	handler.numberOfCalls++
	return true
}

var expectedBashScript = `#!/usr/bin/env bash

mkdir -p ~/.ssh;
touch ~/.ssh/known_hosts;
ssh-keyscan -t rsa,dsa github.com 2>&1 | sort -u - ~/.ssh/known_hosts > ~/.ssh/tmp_hosts;
cat ~/.ssh/tmp_hosts >> ~/.ssh/known_hosts;
` + testutils.TempFileName + ` -i ~/.key clone git@github.com/repo/user;
`

var expectedWrapperScript = `#!/usr/bin/env bash

# The MIT License (MIT)
# Copyright (c) 2013 Alvin Abad

if [ $# -eq 0 ]; then
    echo "Git wrapper script that can specify an ssh-key file
Usage:
    git.sh -i ssh-key-file git-command
    "
    exit 1
fi

# remove temporary file on exit
trap 'rm -f /tmp/.git_ssh.$$' 0

if [ "$1" = "-i" ]; then
    SSH_KEY=$2; shift; shift
    echo -e "#!/usr/bin/env bash\n \
    ssh -i $SSH_KEY \$@" > /tmp/.git_ssh.$$
    chmod +x /tmp/.git_ssh.$$
    export GIT_SSH=/tmp/.git_ssh.$$
fi

# in case the git command is repeated
[ "$1" = "git" ] && shift

# Run the git command
/usr/bin/git "$@"

`

var expectedWrapperScriptAltTmp = `#!/usr/bin/env bash

# The MIT License (MIT)
# Copyright (c) 2013 Alvin Abad

if [ $# -eq 0 ]; then
    echo "Git wrapper script that can specify an ssh-key file
Usage:
    git.sh -i ssh-key-file git-command
    "
    exit 1
fi

# remove temporary file on exit
trap 'rm -f /home/user/tmp/.git_ssh.$$' 0

if [ "$1" = "-i" ]; then
    SSH_KEY=$2; shift; shift
    echo -e "#!/usr/bin/env bash\n \
    ssh -i $SSH_KEY \$@" > /home/user/tmp/.git_ssh.$$
    chmod +x /home/user/tmp/.git_ssh.$$
    export GIT_SSH=/home/user/tmp/.git_ssh.$$
fi

# in case the git command is repeated
[ "$1" = "git" ] && shift

# Run the git command
/usr/bin/git "$@"

`
