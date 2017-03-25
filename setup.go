package git

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

const (
	// DefaultInterval is the minimum interval to delay before
	// requesting another git pull
	DefaultInterval time.Duration = time.Hour * 1
)

func init() {
	caddy.RegisterPlugin("git", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

// setup configures a new Git service routine.
func setup(c *caddy.Controller) error {
	git, err := parse(c)
	if err != nil {
		return err
	}

	// repos configured with webhooks
	var hookRepos []*Repo

	// functions to execute at startup
	var startupFuncs []func() error

	// loop through all repos and and start monitoring
	for i := range git {
		repo := git.Repo(i)

		// If a HookUrl is set, we switch to event based pulling.
		// Install the url handler
		if repo.Hook.URL != "" {

			hookRepos = append(hookRepos, repo)

			startupFuncs = append(startupFuncs, func() error {
				return repo.Pull()
			})

		} else {
			startupFuncs = append(startupFuncs, func() error {

				// Start service routine in background
				Start(repo)

				// Do a pull right away to return error
				return repo.Pull()
			})
		}
	}

	// ensure the functions are executed once per server block
	// for cases like server1.com, server2.com { ... }
	c.OncePerServerBlock(func() error {
		for i := range startupFuncs {
			c.OnStartup(startupFuncs[i])
		}
		return nil
	})

	// if there are repo(s) with webhook
	// return handler
	if len(hookRepos) > 0 {
		webhook := &WebHook{Repos: hookRepos}
		httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
			webhook.Next = next
			return webhook
		})
	}

	return nil
}

func parse(c *caddy.Controller) (Git, error) {
	var git Git

	config := httpserver.GetConfig(c)
	for c.Next() {
		repo := &Repo{Branch: "master", Interval: DefaultInterval, Path: config.Root}

		args := c.RemainingArgs()

		clonePath := func(s string) string {
			if filepath.IsAbs(s) {
				return filepath.Clean(s)
			}
			return filepath.Join(config.Root, s)
		}

		switch len(args) {
		case 2:
			repo.Path = clonePath(args[1])
			fallthrough
		case 1:
			u, err := validateURL(args[0])
			if err != nil {
				return nil, err
			}
			repo.URL = u
		}

		for c.NextBlock() {
			switch c.Val() {
			case "repo":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				u, err := validateURL(c.Val())
				if err != nil {
					return nil, err
				}
				repo.URL = u
			case "path":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				repo.Path = clonePath(c.Val())
			case "branch":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				repo.Branch = c.Val()
			case "key":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				repo.KeyPath = c.Val()
			case "interval":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				t, _ := strconv.Atoi(c.Val())
				if t > 0 {
					repo.Interval = time.Duration(t) * time.Second
				}
			case "args", "clone_args":
				repo.CloneArgs = c.RemainingArgs()
			case "pull_args":
				repo.PullArgs = c.RemainingArgs()
			case "hook":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				repo.Hook.URL = c.Val()

				// optional secret for validation
				if c.NextArg() {
					repo.Hook.Secret = c.Val()
				}
			case "hook_type":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				t := c.Val()
				if _, ok := handlers[t]; !ok {
					return nil, c.Errf("invalid hook type %v", t)
				}
				repo.Hook.Type = t
			case "then":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				command := c.Val()
				args := c.RemainingArgs()
				repo.Then = append(repo.Then, NewThen(command, args...))
			case "then_long":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				command := c.Val()
				args := c.RemainingArgs()
				repo.Then = append(repo.Then, NewLongThen(command, args...))
			default:
				return nil, c.ArgErr()
			}
		}

		// if repo is not specified, return error
		if repo.URL == "" {
			return nil, c.ArgErr()
		}

		// if private key is not specified, convert repository URL to https
		// to avoid ssh authentication
		// else validate git URL
		// Note: private key support not yet available on Windows
		var err error
		if repo.KeyPath == "" {
			repo.URL, repo.Host, err = sanitizeHTTP(repo.URL)
		} else {
			repo.URL, repo.Host, err = sanitizeSSH(repo.URL)
			// TODO add Windows support for private repos
			if runtime.GOOS == "windows" {
				return nil, fmt.Errorf("private repository not yet supported on Windows")
			}
		}

		if err != nil {
			return nil, err
		}

		// validate git requirements
		if err = Init(); err != nil {
			return nil, err
		}

		// prepare repo for use
		if err = repo.Prepare(); err != nil {
			return nil, err
		}

		git = append(git, repo)

	}

	return git, nil
}

// validateURL validates repoUrl is a valid git url and appends
// with .git if missing.
func validateURL(repoURL string) (string, error) {
	u, err := parseURL(repoURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	switch u.Scheme {
	case "https", "http", "ssh":
	default:
		return "", fmt.Errorf("Invalid url scheme %s. If url contains port, scheme is required.", u.Scheme)
	}

	if !strings.HasSuffix(u.String(), ".git") {
		return u.String() + ".git", nil
	}
	return u.String(), nil
}

// sanitizeHTTP cleans up repository URL and converts to https format
// if currently in ssh format.
// Returns sanitized url, hostName (e.g. github.com, bitbucket.com)
// and possible error
func sanitizeHTTP(repoURL string) (string, string, error) {
	u, err := parseURL(repoURL)
	if err != nil {
		return "", "", err
	}

	// ensure the url is not ssh
	if u.Scheme == "ssh" {
		u.Scheme = "https"
	}

	// convert to http format if in ssh format
	if strings.Contains(u.Host, ":") {
		s := strings.SplitN(u.Host, ":", 2)
		//  alter path and host if we're sure its not a port
		if _, err := strconv.Atoi(s[1]); err != nil {
			u.Host = s[0]
			u.Path = path.Join(s[1], u.Path)
		}
	}

	// Bitbucket require the user to be set into the HTTP URL
	if u.Host == "bitbucket.org" && u.User == nil {
		segments := strings.Split(u.Path, "/")
		u.User = url.User(segments[1])
	}

	return u.String(), u.Host, nil
}

// sanitizeSSH cleans up repository url and converts to ssh format for private
// repositories if required.
// Returns sanitized url, hostName (e.g. github.com, bitbucket.com)
// and possible error
func sanitizeSSH(repoURL string) (string, string, error) {
	u, err := parseURL(repoURL)
	if err != nil {
		return "", "", err
	}

	u.Scheme = ""
	host := u.Host
	// convert to ssh format if not in ssh format
	if !strings.Contains(u.Host, ":") {
		if u.Path[0] == '/' {
			u.Path = ":" + u.Path[1:]
		} else if u.Path[0] != ':' {
			u.Path = ":" + u.Path
		}
	} else {
		s := strings.SplitN(u.Host, ":", 2)
		host = s[0]
		// if port is set, ssh scheme is required
		if _, err := strconv.Atoi(s[1]); err == nil {
			u.Scheme = "ssh"
		}
	}

	// ensure user is set
	if u.User == nil {
		u.User = url.User("git")
	}

	// remove unintended `/` added by url.String and `//` if scheme is not ssh.
	// TODO find a cleaner way
	replacer := strings.NewReplacer("/:", ":", "//", "")
	if u.Scheme == "ssh" {
		replacer = strings.NewReplacer("/:", ":")
	}
	repoURL = replacer.Replace(u.String())
	return repoURL, host, nil
}

func parseURL(repoURL string) (*url.URL, error) {
	var replacers struct {
		before, after *strings.Replacer
	}
	replacers.before = strings.NewReplacer(":", "..")
	replacers.after = strings.NewReplacer("..", ":")

	// hack to hide colons in ssh URL
	if str := strings.Split(repoURL, "://"); len(str) > 1 {
		repoURL = str[0] + "://" + replacers.before.Replace(str[1])
	} else {
		repoURL = replacers.before.Replace(repoURL)
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}
	u.Host = replacers.after.Replace(u.Host)
	return u, nil
}
