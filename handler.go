package git

import (
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// Handler is middleware for handling web hooks and other endpoints of git providers
type Handler struct {
	Repos []*Repo
	Next  httpserver.Handler
}

var endpointHandlers = []func(http.ResponseWriter, *http.Request, *Repo) (bool, int, error){
	handleWebhookIfRequired,
	handleStatusEndpointIfRequired,
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	for _, repo := range h.Repos {
		for _, endpointHandler := range endpointHandlers {
			handled, status, err := endpointHandler(w, r, repo)
			if handled {
				return status, err
			}
		}
	}
	return h.Next.ServeHTTP(w, r)
}
