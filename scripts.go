package git

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/abiosoft/caddy-git/gitos"
)

var (
	// gitBinary holds the absolute path to git executable
	gitBinary string

	// shell holds the shell to be used. Either sh or bash.
	shell string

	// initMutex prevents parallel attempt to validate
	// git requirements.
	initMutex = sync.Mutex{}
)

// Init validates git installation, locates the git executable
// binary in PATH and check for available shell to use.
func Init() error {
	// prevent concurrent call
	initMutex.Lock()
	defer initMutex.Unlock()

	// if validation has been done before and binary located in
	// PATH, return.
	if gitBinary != "" {
		return nil
	}

	// locate git binary in path
	var err error
	if gitBinary, err = gos.LookPath("git"); err != nil {
		return fmt.Errorf("git middleware requires git installed. Cannot find git binary in PATH")
	}

	// locate bash in PATH. If not found, fallback to sh.
	// If neither is found, return error.
	shell = "bash"
	if _, err = gos.LookPath("bash"); err != nil {
		shell = "sh"
		if _, err = gos.LookPath("sh"); err != nil {
			return fmt.Errorf("git middleware requires either bash or sh.")
		}
	}
	return nil
}

// runCmd is a helper function to run commands.
// It runs command with args from directory at dir.
// The executed process outputs to os.Stderr
func runCmd(command string, args []string, dir string) error {
	cmd := gos.Command(command, args...)
	cmd.Stdout(os.Stderr)
	cmd.Stderr(os.Stderr)
	cmd.Dir(dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// runCmdOutput is a helper function to run commands and return output.
// It runs command with args from directory at dir.
// If successful, returns output and nil error
func runCmdOutput(command string, args []string, dir string) (string, error) {
	cmd := gos.Command(command, args...)
	cmd.Dir(dir)
	var err error
	if output, err := cmd.Output(); err == nil {
		return string(bytes.TrimSpace(output)), nil
	}
	return "", err
}

// writeScriptFile writes content to a temporary file.
// It changes the temporary file mode to executable and
// closes it to prepare it for execution.
func writeScriptFile(content []byte) (file gitos.File, err error) {
	if file, err = gos.TempFile("", "caddy"); err != nil {
		return nil, err
	}
	if _, err = file.Write(content); err != nil {
		return nil, err
	}
	if err = file.Chmod(os.FileMode(0755)); err != nil {
		return nil, err
	}
	return file, file.Close()
}

// gitWrapperScript forms content for git.sh script
func gitWrapperScript() []byte {
	return []byte(fmt.Sprintf(`#!/bin/%v

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
    echo -e "#!/bin/%v \n \
    ssh -i $SSH_KEY \$@" > /tmp/.git_ssh.$$
    chmod +x /tmp/.git_ssh.$$
    export GIT_SSH=/tmp/.git_ssh.$$
fi

# in case the git command is repeated
[ "$1" = "git" ] && shift

# Run the git command
%v "$@"

`, shell, shell, gitBinary))
}

// bashScript forms content of bash script to clone or update a repo using ssh
func bashScript(gitShPath string, repo *Repo, params []string) []byte {
	return []byte(fmt.Sprintf(`#!/bin/%v

mkdir -p ~/.ssh;
touch ~/.ssh/known_hosts;
ssh-keyscan -t rsa,dsa %v 2>&1 | sort -u - ~/.ssh/known_hosts > ~/.ssh/tmp_hosts;
cat ~/.ssh/tmp_hosts >> ~/.ssh/known_hosts;
%v -i %v %v;
`, shell, repo.Host, gitShPath, repo.KeyPath, strings.Join(params, " ")))
}
