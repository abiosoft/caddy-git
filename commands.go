package git

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"time"
)

// Then is the command executed after successful pull.
type Then interface {
	Command() string
	Exec(string) error
}

// NewThen creates a new Then command.
func NewThen(command string, args ...string) Then {
	return &gitCmd{command: command, args: args}
}

// NewLongThen creates a new long running Then comand.
func NewLongThen(command string, args ...string) Then {
	return &gitCmd{command: command, args: args, background: true, haltChan: make(chan struct{})}
}

type gitCmd struct {
	command    string
	args       []string
	dir        string
	background bool
	process    *os.Process

	haltChan   chan struct{}
	monitoring bool
	sync.RWMutex
}

// Command returns the full command as configured in Caddyfile
func (g *gitCmd) Command() string {
	return g.command + " " + strings.Join(g.args, " ")
}

// Exec executes the command initiated in GitCmd
func (g *gitCmd) Exec(dir string) error {
	g.Lock()
	g.dir = dir
	g.Unlock()

	if g.background {
		return g.execBackground(dir)
	}
	return g.exec(dir)
}

func (g *gitCmd) restart() error {
	err := g.Exec(g.dir)
	if err == nil {
		Logger().Printf("Restart successful for '%v'.\n", g.Command())
	} else {
		Logger().Printf("Restart failed for '%v'.\n", g.Command())
	}
	return err
}

func (g *gitCmd) exec(dir string) error {
	return runCmd(g.command, g.args, dir)
}

func (g *gitCmd) execBackground(dir string) error {
	// if existing process is running, kill it.
	g.RLock()
	if g.process != nil {
		g.haltProcess()
	}
	g.RUnlock()

	process, err := runCmdBackground(g.command, g.args, dir)
	if err == nil {
		g.Lock()
		g.process = process
		g.Unlock()
		g.monitorProcess()
	}
	return err
}

func (g *gitCmd) monitorProcess() {
	g.RLock()
	if g.process == nil {
		g.RUnlock()
		return
	}
	monitoring := g.monitoring
	g.RUnlock()

	if monitoring {
		return
	}

	type resp struct {
		state *os.ProcessState
		err   error
	}

	respChan := make(chan resp)

	go func() {
		p, err := g.process.Wait()
		respChan <- resp{p, err}
	}()

	go func() {
		g.Lock()
		g.monitoring = true
		g.Unlock()

		select {
		case <-g.haltChan:
			g.killProcess()
		case r := <-respChan:
			if r.err != nil || !r.state.Success() {
				Logger().Printf("Command '%v' terminated with error", g.Command())

				g.Lock()
				g.process = nil
				g.monitoring = false
				g.Unlock()

				for i := 0; ; i++ {
					if i >= numRetries {
						Logger().Printf("Restart failed after 3 attempts for '%v'. Ignoring...\n", g.Command())
						break
					}
					Logger().Printf("Attempting restart %v of %v for '%v'\n", i+1, numRetries, g.Command())
					if g.restart() == nil {
						break
					}
					time.Sleep(time.Second * 5)
				}
			} else {
				g.Lock()
				g.process = nil
				g.monitoring = false
				g.Unlock()
			}
		}
	}()

}

func (g *gitCmd) killProcess() {
	g.Lock()
	if err := g.process.Kill(); err != nil {
		Logger().Printf("Could not terminate running command '%v'\n", g.command)
	} else {
		Logger().Printf("Command '%v' terminated from within.\n", g.command)
	}
	g.process = nil
	g.monitoring = false
	g.Unlock()
}

// haltProcess halts the running process
func (g *gitCmd) haltProcess() {
	g.RLock()
	monitoring := g.monitoring
	g.RUnlock()

	if monitoring {
		g.haltChan <- struct{}{}
	}
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
