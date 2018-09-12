package git

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// WebHook is middleware for handling web hooks of git providers
type WebHook struct {
	Repos []*Repo
	Next  httpserver.Handler
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
	"gitee":     GiteeHook{},
}

// defaultHandlers is the list of handlers to choose from
// if handler type is not specified in config.
var defaultHandlers = []string{
	"github",
	"gitlab",
	"bitbucket",
	"travis",
	"gogs",
	"gitee",
}

// ServeHTTP implements the middlware.Handler interface.
func (h WebHook) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	for _, repo := range h.Repos {

		if r.URL.Path == repo.Hook.URL {

			// if handler type is specified.
			if handler, ok := handlers[repo.Hook.Type]; ok {
				if !handler.DoesHandle(r.Header) {
					return http.StatusBadRequest, errors.New(http.StatusText(http.StatusBadRequest))
				}
				status, err := handler.Handle(w, r, repo)
				// if the webhook is ignored, log it and allow request to continue.
				if hookIgnored(err) {
					Logger().Println(err)
					err = nil
				}
				return status, err
			}

			// auto detect handler
			for _, h := range defaultHandlers {
				// if a handler indicates it does handle the request,
				// we do not try other handlers. Only one handler ever
				// handles a specific request.
				if handlers[h].DoesHandle(r.Header) {
					status, err := handlers[h].Handle(w, r, repo)
					// if the webhook is ignored, log it and allow request to continue.
					if hookIgnored(err) {
						Logger().Println(err)
						err = nil
					}
					return status, err
				}
			}

			// no compatible handler
			Logger().Println("No compatible handler found. Consider enabling generic handler with 'hook_type generic'.")
		}
	}

	return h.Next.ServeHTTP(w, r)
}
