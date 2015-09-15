package git

import (
	"net/http"

	"github.com/mholt/caddy/middleware"
)

// Middleware for handling web hooks of git providers
type WebHook struct {
	Repo *Repo
	Next middleware.Handler
}

// Interface for specific providers to implement.
type hookHandler interface {
	DoesHandle(http.Header) bool
	Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error)
}

// Slice of all registered hookHandlers.
// Register new hook handlers here!
var handlers = []hookHandler{
	GithubHook{},
	GitlabHook{},
	BitbucketHook{},
}

// ServeHTTP implements the middlware.Handler interface.
func (h WebHook) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	if r.URL.Path == h.Repo.HookUrl {

		for _, handler := range handlers {
			// if a handler indicates it does handle the request,
			// we do not try other handlers. Only one handler ever
			// handles a specific request.
			if handler.DoesHandle(r.Header) {
				return handler.Handle(w, r, h.Repo)
			}
		}
	}

	return h.Next.ServeHTTP(w, r)
}
