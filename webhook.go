package git

import (
	"fmt"
	"net/http"

	"github.com/miekg/dns"
	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
)

// WebHook is middleware for handling web hooks of git providers
type WebHook struct {
	Repos []*Repo
	Next  middleware.Handler
}

// HookConfig is a webhook handler configuration.
type HookConfig struct {
	URL    string // url to listen on for webhooks
	Secret string // secret to validate hooks
	Type   string // type of Webhook
}

// hookIgnoredError is returned when a webhook is ignored by the
// webhook handler.
type hookIgnoredError struct {
	hookType string
	err      error
}

// Error satisfies error interface
func (h hookIgnoredError) Error() string {
	return fmt.Sprintf("%s webhook ignored. Error: %v", h.hookType, h.err)
}

// hookIgnored checks if err is of type hookIgnoredError.
func hookIgnored(err error) bool {
	_, ok := err.(hookIgnoredError)
	return ok
}

// hookName returns the name of the hookHanlder h.
func hookName(h hookHandler) string {
	for name, handler := range handlers {
		if handler == h {
			return name
		}
	}
	return ""
}

// hookHandler is interface for specific providers to implement.
type hookHandler interface {
	DoesHandle(http.Header) bool
	Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error)
}

// handlers stores all registered hookHandlers.
// map key corresponds to expected config name.
//
// register hook handlers here.
var handlers = map[string]hookHandler{
	"github":    GithubHook{},
	"gitlab":    GitlabHook{},
	"bitbucket": BitbucketHook{},
	"generic":   GenericHook{},
	"travis":    TravisHook{},
	"gogs":      GogsHook{},
}

// defaultHandlers is the list of handlers to choose from
// if handler type is not specified in config.
var defaultHandlers = []string{
	"github",
	"gitlab",
	"bitbucket",
	"travis",
	"gogs",
}

// ServeDNS implements the middlware.Handler interface.
func (h WebHook) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// We don't support web hooks. What am I? A webserver?
	return h.Next.ServeDNS(ctx, w, r)
}

func (h WebHook) Name() string { return "git" }
