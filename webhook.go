package git

import (
	"errors"
	"net/http"

	"github.com/mholt/caddy/middleware"
)

// Middleware for handling web hooks of git providers
type WebHook struct {
	Repos []*Repo
	Next  middleware.Handler
}

// HookConfig is a webhook handler configuration.
type HookConfig struct {
	Url    string // url to listen on for webhooks
	Secret string // secret to validate hooks
	Type   string // type of Webhook
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
}

// defaultHandlers is the list of handlers to choose from
// if handler type is not specified in config.
var defaultHandlers = []hookHandler{
	GithubHook{},
	GitlabHook{},
	BitbucketHook{},
	TravisHook{},
}

// registerHandler registers hook handler.
// call on init() to register hook handler.
func registerHandler(name string, handler hookHandler) {
	handlers[name] = handler
}

// ServeHTTP implements the middlware.Handler interface.
func (h WebHook) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	for _, repo := range h.Repos {

		if r.URL.Path == repo.Hook.Url {

			// if handler type is specified.
			if handler, ok := handlers[repo.Hook.Type]; ok {
				if !handler.DoesHandle(r.Header) {
					return http.StatusBadRequest, errors.New(http.StatusText(http.StatusBadRequest))
				}
				return handler.Handle(w, r, repo)
			}

			// auto detect handler
			for _, handler := range defaultHandlers {
				// if a handler indicates it does handle the request,
				// we do not try other handlers. Only one handler ever
				// handles a specific request.
				if handler.DoesHandle(r.Header) {
					return handler.Handle(w, r, repo)
				}
			}
		}
	}

	return h.Next.ServeHTTP(w, r)
}
