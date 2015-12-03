package git

import (
	"bytes"
	"os"
	"strings"
)

// Then is the command executed after successful pull.
type Then interface {
	Command() string
	Exec() error
}

type gitCmd struct {
	command    string
	args       []string
	dir        string
	background bool
	process    *os.Process
}

// Command returns the full command as configured in Caddyfile
func (g *gitCmd) Command() string {
	return g.command + " " + strings.Join(g.args, " ")
}

// Exec executes the command initiated in GitCmd
func (g *gitCmd) Exec() error {
	if g.background {
		return g.execBackground()
	}
	return g.exec()
}

func (g *gitCmd) exec() error {
	return runCmd(g.command, g.args, g.dir)
}

func (g *gitCmd) execBackground() error {
	// if existing process is running, kill it.
	if g.process != nil {
		if err := g.process.Kill(); err != nil {
			Logger().Printf("Could not terminate running command %v\n", g.command)
		} else {
			Logger().Printf("Running command %v terminated.\n", g.command)
		}
	}
	process, err := runCmdBackground(g.command, g.args, g.dir)
	if err == nil {
		g.process = process
	}
	return err
}

// NewThen creates a new Then command.
func NewThen(command string, args ...string) Then {
	return &gitCmd{command: command, args: args}
}

// NewLongThen creates a new long running Then comand.
func NewLongThen(command string, args ...string) Then {
	return &gitCmd{command: command, args: args, background: true}
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

// runCmdBackground is a helper function to run commands in the background.
// It returns the resulting process and an error that occurs during while
// starting the process (if any).
func runCmdBackground(command string, args []string, dir string) (*os.Process, error) {
	cmd := gos.Command(command, args...)
	cmd.Dir(dir)
	cmd.Stdout(os.Stderr)
	cmd.Stderr(os.Stderr)
	err := cmd.Start()
	return cmd.Process(), err
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
