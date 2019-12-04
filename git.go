package git

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/abiosoft/caddy-git/gitos"
	"github.com/caddyserver/caddy"
)

const (
	// Number of retries if git pull fails
	numRetries = 3

	// variable for latest tag
	latestTag = "{latest}"
)

// Git represent multiple repositories.
type Git []*Repo

// Repo retrieves repository at i or nil if not found.
func (g Git) Repo(i int) *Repo {
	if i < len(g) {
		return g[i]
	}
	return nil
}

// RepoURL is the repository url.
type RepoURL string

// String satisfies stringer and attempts to strip off authentication
// info from url if it exists.
func (r RepoURL) String() string {
	u, err := url.Parse(string(r))
	if err != nil {
		return string(r)
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// Val returns git friendly Val that can be
// passed to git clone.
func (r RepoURL) Val() string {
	if strings.HasPrefix(string(r), "ssh://") {
		return strings.TrimPrefix(string(r), "ssh://")
	}
	return string(r)
}

// Repo is the structure that holds required information
// of a git repository.
type Repo struct {
	URL         RepoURL       // Repository URL
	Path        string        // Directory to pull to
	Host        string        // Git domain host e.g. github.com
	Branch      string        // Git branch
	KeyPath     string        // Path to private ssh key
	Interval    time.Duration // Interval between pulls
	CloneArgs   []string      // Additonal cli args to pass to git clone
	PullArgs    []string      // Additonal cli args to pass to git pull
	SkipStartup bool          // true if the initial pull/clone should not be be executed on startup
	Then        []Then        // Commands to execute after successful git pull
	pulled      bool          // true if there was a successful pull
	lastPull    time.Time     // time of the last successful pull
	lastCommit  string        // hash for the most recent commit
	latestTag   string        // latest tag name
	Hook        HookConfig    // Webhook configuration
	sync.Mutex
}

// Pull attempts a git pull.
// It retries at most numRetries times if error occurs
func (r *Repo) Pull() error {
	r.Lock()
	defer r.Unlock()

	// prevent a pull if the last one was less than 5 seconds ago
	if gos.TimeSince(r.lastPull) < 5*time.Second {
		return nil
	}

	// keep last commit hash for comparison later
	lastCommit := r.lastCommit

	var err error
	// Attempt to pull at most numRetries times
	for i := 0; i < numRetries; i++ {
		if err = r.pull(); err == nil {
			break
		}
		Logger().Println(err)
	}

	if err != nil {
		return err
	}

	// check if there are new changes,
	// then execute post pull command
	if r.lastCommit == lastCommit {
		Logger().Println("No new changes.")
		return nil
	}
	return r.execThen()
}

// pull performs git pull, or git clone if repository does not exist.
func (r *Repo) pull() error {

	// if not pulled, perform clone
	if !r.pulled {
		return r.clone()
	}

	// if latest tag config is set
	if r.Branch == latestTag {
		return r.checkoutLatestTag()
	}

	params := append([]string{"pull"}, append(r.PullArgs, "origin", r.Branch)...)
	var err error
	if err = r.gitCmd(params, r.Path); err == nil {
		r.pulled = true
		r.lastPull = time.Now()
		Logger().Printf("%v pulled.\n", r.URL)
		r.lastCommit, err = r.mostRecentCommit()
	}
	return err
}

// clone performs git clone.
func (r *Repo) clone() error {
	params := append([]string{"clone", "-b", r.Branch}, append(r.CloneArgs, r.URL.Val(), r.Path)...)

	tagMode := r.Branch == latestTag
	if tagMode {
		params = append([]string{"clone"}, append(r.CloneArgs, r.URL.Val(), r.Path)...)
	}

	var err error
	if err = r.gitCmd(params, ""); err == nil {
		r.pulled = true
		r.lastPull = time.Now()
		Logger().Printf("%v pulled.\n", r.URL)
		r.lastCommit, err = r.mostRecentCommit()

		// if latest tag config is set.
		if tagMode {
			return r.checkoutLatestTag()
		}
	}

	return err
}

// checkoutLatestTag checks out the latest tag of the repository.
func (r *Repo) checkoutLatestTag() error {
	tag, err := r.fetchLatestTag()
	if err != nil {
		Logger().Println("Error retrieving latest tag.")
		return err
	}
	if tag == "" {
		Logger().Println("No tags found for Repo: ", r.URL)
		return fmt.Errorf("No tags found for Repo: %v", r.URL)
	} else if tag == r.latestTag {
		Logger().Println("No new tags.")
		return nil
	}

	params := []string{"checkout", "tags/" + tag}
	if err = r.gitCmd(params, r.Path); err == nil {
		r.latestTag = tag
		r.lastCommit, err = r.mostRecentCommit()
		Logger().Printf("Tag %v checkout done.\n", tag)
	}
	return err
}

// checkoutCommit checks out the specified commitHash.
func (r *Repo) checkoutCommit(commitHash string) error {
	var err error
	params := []string{"checkout", commitHash}
	if err = r.gitCmd(params, r.Path); err == nil {
		Logger().Printf("Commit %v checkout done.\n", commitHash)
	}
	return err
}

// gitCmd performs a git command.
func (r *Repo) gitCmd(params []string, dir string) error {
	// if key is specified, use ssh key
	if r.KeyPath != "" {
		return r.gitCmdWithKey(params, dir)
	}
	return runCmd(gitBinary, params, dir)
}

// gitCmdWithKey is used for private repositories and requires an ssh key.
// Note: currently only limited to Linux and OSX.
func (r *Repo) gitCmdWithKey(params []string, dir string) error {
	var gitSSH, script gitos.File
	// ensure temporary files deleted after usage
	defer func() {
		if gitSSH != nil {
			gos.Remove(gitSSH.Name())
		}
		if script != nil {
			gos.Remove(script.Name())
		}
	}()

	var err error
	// write git.sh script to temp file
	gitSSH, err = writeScriptFile(gitWrapperScript())
	if err != nil {
		return err
	}

	// write git bash script to file
	script, err = writeScriptFile(bashScript(gitSSH.Name(), r, params))
	if err != nil {
		return err
	}

	return runCmd(script.Name(), nil, dir)
}

// Prepare prepares for a git pull
// and validates the configured directory
func (r *Repo) Prepare() error {
	// check if directory exists or is empty
	// if not, create directory
	fs, err := gos.ReadDir(r.Path)
	if err != nil || len(fs) == 0 {
		return gos.MkdirAll(r.Path, os.FileMode(0755))
	}

	// validate git repo
	isGit := false
	for _, f := range fs {
		if f.IsDir() && f.Name() == ".git" {
			isGit = true
			break
		}
	}

	if isGit {
		// check if same repository
		var repoURL string
		if repoURL, err = r.originURL(); err == nil {
			if strings.TrimSuffix(repoURL, ".git") == strings.TrimSuffix(r.URL.Val(), ".git") {
				r.pulled = true
				return nil
			}
		}
		if err != nil {
			return fmt.Errorf("cannot retrieve repo url for %v Error: %v", r.Path, err)
		}
		return fmt.Errorf("another git repo '%v' exists at %v", repoURL, r.Path)
	}
	return fmt.Errorf("cannot git clone into %v, directory not empty", r.Path)
}

// getMostRecentCommit gets the hash of the most recent commit to the
// repository. Useful for checking if changes occur.
func (r *Repo) mostRecentCommit() (string, error) {
	command := gitBinary + ` --no-pager log -n 1 --pretty=format:"%H"`
	c, args, err := caddy.SplitCommandAndArgs(command)
	if err != nil {
		return "", err
	}
	return runCmdOutput(c, args, r.Path)
}

// fetchLatestTag retrieves the most recent tag in the repository.
func (r *Repo) fetchLatestTag() (string, error) {
	// fetch updates to get latest tag
	params := []string{"fetch", "origin", "--tags"}
	err := r.gitCmd(params, r.Path)
	if err != nil {
		return "", err
	}
	// retrieve latest tag
	command := gitBinary + ` describe origin --abbrev=0 --tags`
	c, args, err := caddy.SplitCommandAndArgs(command)
	if err != nil {
		return "", err
	}
	return runCmdOutput(c, args, r.Path)
}

// originURL retrieves remote origin url for the git repository at path
func (r *Repo) originURL() (string, error) {
	_, err := gos.Stat(r.Path)
	if err != nil {
		return "", err
	}
	args := []string{"config", "--get", "remote.origin.url"}
	return runCmdOutput(gitBinary, args, r.Path)
}

// execThen executes r.Then.
// It is trigged after successful git pull
func (r *Repo) execThen() error {
	var errs error
	for _, command := range r.Then {
		err := command.Exec(r.Path)
		if err == nil {
			Logger().Printf("Command '%v' successful.\n", command.Command())
		}
		errs = mergeErrors(errs, err)
	}
	return errs
}

func mergeErrors(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	var err error
	for _, e := range errs {
		if err == nil {
			err = e
			continue
		}
		if e != nil {
			err = fmt.Errorf("%v\n%v", err.Error(), e.Error())
		}
	}
	return err
}
