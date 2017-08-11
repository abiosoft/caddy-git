package git

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type StatusEndpointSkill string

const (
	statusEndpointSkillGet       = "get"
	statusEndpointSkillWebsocket = "websocket"
)

var supportedStatusEndpointSkills = []StatusEndpointSkill{
	statusEndpointSkillGet,
	statusEndpointSkillWebsocket,
}

type StatusEndpointConfig struct {
	URL    string                       // url to listen on
	Secret string                       // secret to validate requests
	Skills map[StatusEndpointSkill]bool // list of activated skills for this endpoint
}

func (config StatusEndpointConfig) isSkillEnabled(skill StatusEndpointSkill) bool {
	if config.Skills == nil {
		return true
	}
	val, ok := config.Skills[skill]
	if !ok {
		return true
	}
	return val
}

type statusChangeHandler interface {
	handle(*Repo, Status) bool
}

type statusChangeHandlerImpl struct {
	ws *websocket.Conn
}

func (handler *statusChangeHandlerImpl) handle(repo *Repo, status Status) bool {
	bytes, err := json.Marshal(status)
	if err != nil {
		Logger().Printf("Could not marshal status. Got: '%v'\n", err)
		return true
	}
	err = handler.ws.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		Logger().Printf("It was not possible to write to client. Got: '%v'\n", err)
		return false
	}
	return true
}

type statusEndpointState struct {
	sync.Mutex
	onStatusChange *map[statusChangeHandler]bool
}

func newStatusEndpointState() *statusEndpointState {
	result := new(statusEndpointState)
	result.onStatusChange = &map[statusChangeHandler]bool{}
	return result
}

func (state *statusEndpointState) registerOnStatusChange(handler statusChangeHandler) error {
	state.Lock()
	defer state.Unlock()
	(*state.onStatusChange)[handler] = true
	return nil
}

func (state *statusEndpointState) unregisterOnStatusChange(handler statusChangeHandler) error {
	state.Lock()
	defer state.Unlock()
	delete(*state.onStatusChange, handler)
	return nil
}

func (state *statusEndpointState) fireStatusChangeEvent(repo *Repo, status Status) error {
	state.Lock()
	defer state.Unlock()
	toRemove := map[statusChangeHandler]bool{}
	for handler := range *state.onStatusChange {
		if !handler.handle(repo, status) {
			toRemove[handler] = true
		}
	}

	for handler := range toRemove {
		delete(*state.onStatusChange, handler)
	}

	return nil
}

func handleStatusEndpointIfRequired(w http.ResponseWriter, r *http.Request, repo *Repo) (bool, int, error) {
	if repo.StatusEndpoint.URL != "" && r.URL.Path == repo.StatusEndpoint.URL {
		return handleStatusEndpoint(w, r, repo)
	}
	return false, 0, nil
}

func handleStatusEndpoint(w http.ResponseWriter, r *http.Request, repo *Repo) (bool, int, error) {
	if repo.StatusEndpoint.Secret != "" {
		u, p, ok := r.BasicAuth()
		if !ok || (u != repo.StatusEndpoint.Secret && p != repo.StatusEndpoint.Secret) {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Secure Area\"")
			http.Error(w, "No or invalid secret.", http.StatusUnauthorized)
			return true, 0, nil
		}
	}

	if r.Method == "GET" && r.Header.Get("Upgrade") == "websocket" {
		err := handleStatusEndpointAsWebsocket(w, r, repo)
		return true, 0, err
	}

	if r.Method == "GET" {
		err := handleStatusEndpointAsGet(w, r, repo)
		return true, 0, err
	}

	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return true, 0, nil
}

func handleStatusEndpointAsWebsocket(w http.ResponseWriter, r *http.Request, repo *Repo) (error) {
	if !repo.StatusEndpoint.isSkillEnabled(statusEndpointSkillWebsocket) {
		http.Error(w, "Requesting this endpoint as websocket not allowed.", http.StatusMethodNotAllowed)
		return nil
	}

	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	handler := &statusChangeHandlerImpl{
		ws: ws,
	}
	repo.registerOnStatusChange(handler)
	go func() {
		defer ws.Close()
		for {
			// Continue pulling messages to detect if the client disconnected but do not really care
			// what to clients sends...
			t, _, err := ws.ReadMessage()
			if err != nil || t == websocket.CloseMessage {
				repo.unregisterOnStatusChange(handler)
				break
			}
		}
	}()

	return nil
}

func handleStatusEndpointAsGet(w http.ResponseWriter, r *http.Request, repo *Repo) (error) {
	if !repo.StatusEndpoint.isSkillEnabled(statusEndpointSkillGet) {
		http.Error(w, "Requesting this endpoint as regular HTTP GET not allowed.", http.StatusMethodNotAllowed)
		return nil
	}

	status := repo.toStatus()
	js, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	return nil
}
