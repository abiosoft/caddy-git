package git

import (
	"net/http"
	"net/url"
	"testing"
	"github.com/abiosoft/caddy-git/testutils"
	"time"
)

func TestHandlerWithoutAnyInterception(t *testing.T) {
	handlers := testHandlersFor(t, &Repo{
	})
	r := testRequestFor("GET", "/foo")

	executeAndCheckHandler(handlers, r, 0, 1)
}

func TestHandlerWithInterceptionByStatusEndpoint(t *testing.T) {
	handlers := testHandlersFor(t, &Repo{
		pulled:     true,
		lastCommit: "abc",
		StatusEndpoint: StatusEndpointConfig{
			URL: "/foo",
		},
	})
	r := testRequestFor("GET", "/foo")

	w := executeAndCheckHandler(handlers, r, 0, 0)
	if w.Written.String() != expectedBodyOfCallingStatusEndpoint {
		t.Errorf("Expected: %v,\nBut got: %v", expectedBodyOfCallingStatusEndpoint, w.Written.String())
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected: %v, But got: %v", http.StatusOK, w.Code)
	}
}

func TestHandlerWithBlockedInterceptionOfStatusEndpoint(t *testing.T) {
	handlers := testHandlersFor(t, &Repo{
		pulled:     true,
		lastCommit: "abc",
		StatusEndpoint: StatusEndpointConfig{
			URL: "/foo",
			Skills: map[StatusEndpointSkill]bool{
				statusEndpointSkillGet: false,
			},
		},
	})
	r := testRequestFor("GET", "/foo")

	w := executeAndCheckHandler(handlers, r, 0, 0)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected: %v, But got: %v", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandlerWithInterceptionOfStatusEndpointWebsocket(t *testing.T) {
	handlers := testHandlersFor(t, &Repo{
		pulled:     true,
		lastCommit: "abc",
		StatusEndpoint: StatusEndpointConfig{
			URL: "/foo",
		},
		seState: newStatusEndpointState(),
	})
	r := testRequestFor("GET", "/foo")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Sec-WebSocket-Key", "EVIJMwiWyx6bVnpdnFUEYw==")
	r.Header.Set("Sec-WebSocket-Version", "13")

	w := executeAndCheckHandler(handlers, r, 0, 0)
	if w.Written.String() != "" {
		t.Errorf("Expected: %v, But got: %v", "", w.Written.String())
	}
	if w.Code != -1 {
		t.Errorf("Expected: %v, But got: %v", -1, w.Code)
	}
	if len(*handlers.repo.seState.onStatusChange) != 1 {
		t.Errorf("Expected: %v, But got: %v\n\tContent: %v", 1, len(*handlers.repo.seState.onStatusChange), handlers.repo.seState.onStatusChange)
	}
	if len(w.Connections) != 1 {
		t.Errorf("Expected: %v, But got: %v\n\tContent: %v", 1, len(w.Connections), w.Connections)
	}
	w.Connections[0].Close()
	// Wait that the connection will be cleaned up in the parallel go routine.
	time.Sleep(100 * time.Millisecond)
	if len(*handlers.repo.seState.onStatusChange) != 0 {
		t.Errorf("Expected: %v, But got: %v\n\tContent: %v", 0, len(*handlers.repo.seState.onStatusChange), handlers.repo.seState.onStatusChange)
	}
}

func TestHandlerWithBlockedInterceptionOfStatusEndpointWebsocket(t *testing.T) {
	handlers := testHandlersFor(t, &Repo{
		pulled:     true,
		lastCommit: "abc",
		StatusEndpoint: StatusEndpointConfig{
			URL: "/foo",
			Skills: map[StatusEndpointSkill]bool{
				statusEndpointSkillWebsocket: false,
			},
		},
		seState: newStatusEndpointState(),
	})
	r := testRequestFor("GET", "/foo")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Sec-WebSocket-Key", "EVIJMwiWyx6bVnpdnFUEYw==")
	r.Header.Set("Sec-WebSocket-Version", "13")

	w := executeAndCheckHandler(handlers, r, 0, 0)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected: %v, But got: %v", http.StatusMethodNotAllowed, w.Code)
	}
}

func executeAndCheckHandler(handlers *testHandlers, r *http.Request, expectedCode int, expectedNumberOfNextCalls int) *testutils.MockResponseWriter {
	w := testutils.NewMockResponseWriter()
	code, err := handlers.handler.ServeHTTP(w, r)
	if code != expectedCode {
		handlers.t.Errorf("Expected: %v, But got: %v", expectedCode, code)
	}
	check(handlers.t, err)
	if handlers.nextHandler.numberOfCalls != expectedNumberOfNextCalls {
		handlers.t.Errorf("Expected: %v, But got: %v", expectedNumberOfNextCalls, handlers.nextHandler.numberOfCalls)
	}
	return w
}

func testRequestFor(method string, path string) *http.Request {
	return &http.Request{
		Method: method,
		Header: http.Header{},
		URL: &url.URL{
			Path: path,
		},
	}
}

func testHandlersFor(t *testing.T, repo *Repo) *testHandlers {
	nextHandler := &testHttpHandler{}
	handler := &Handler{
		Repos: []*Repo{repo},
		Next:  nextHandler,
	}
	return &testHandlers{
		repo:        repo,
		t:           t,
		handler:     handler,
		nextHandler: nextHandler,
	}
}

type testHandlers struct {
	repo        *Repo
	t           *testing.T
	handler     *Handler
	nextHandler *testHttpHandler
}

type testHttpHandler struct {
	numberOfCalls int
}

func (handler *testHttpHandler) ServeHTTP(http.ResponseWriter, *http.Request) (int, error) {
	handler.numberOfCalls++
	return 0, nil
}

const (
	expectedBodyOfCallingStatusEndpoint = `{"active":true,"hash":"abc","last_updated":"0001-01-01T00:00:00Z"}`
)
