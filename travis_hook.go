package git

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type TravisHook struct{}

func (t TravisHook) DoesHandle(h http.Header) bool {
	return h.Get("Travis-Repo-Slug") != ""
}

func (t TravisHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method")
	}
	if err := t.handleSignature(r, repo.Hook.Secret); err != nil {
		return http.StatusBadRequest, err
	}
	if err := r.ParseForm(); err != nil {
		return http.StatusBadRequest, err
	}
	payload := r.FormValue("payload")
	if payload == "" {
		return http.StatusBadRequest, fmt.Errorf("Payload required")
	}
	data := &travisPayload{}
	if err := json.Unmarshal([]byte(payload), data); err != nil {
		return http.StatusBadRequest, err
	}
	if data.Type != "push" || data.StatusMessage != "Passed" {
		Logger().Println("Ignoring payload with wrong status or type.")
		return 200, nil
	}
	if repo.Branch != "" && data.Branch != repo.Branch {
		Logger().Printf("Ignoring push for branch %s", data.Branch)
		return 200, nil
	}
	if err := repo.Pull(); err != nil {
		return http.StatusInternalServerError, err
	}
	if err := repo.checkoutCommit(data.Commit); err != nil {
		return http.StatusInternalServerError, err
	}
	return 200, nil
}

type travisPayload struct {
	ID            int       `json:"id"`
	Number        string    `json:"number"`
	Status        int       `json:"status"`
	Result        int       `json:"result"`
	StatusMessage string    `json:"status_message"`
	ResultMessage string    `json:"result_message"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Duration      int       `json:"duration"`
	BuildURL      string    `json:"build_url"`
	Branch        string    `json:"branch"`
	Type          string    `json:"type"`
	State         string    `json:"state"`
	Commit        string    `json:"commit"`
}

// Check for an authorization signature in the request. Reject if not present. If validation required, check the sha
func (t TravisHook) handleSignature(r *http.Request, secret string) error {
	signature := r.Header.Get("Authorization")
	if signature == "" {
		return errors.New("request sent no authorization signature")
	}
	if secret == "" {
		Logger().Print("Unable to verify request signature. Secret not set in caddyfile!\n")
		return nil
	}

	content := r.Header.Get("Travis-Repo-Slug") + secret
	hash := sha256.Sum256([]byte(content))
	expectedMac := hex.EncodeToString(hash[:])
	if signature != expectedMac {
		fmt.Println(signature, expectedMac)
		return errors.New("Invalid authorization header")
	}
	return nil
}
