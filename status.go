package git

import (
	"errors"
	"time"
)

type Status struct {
	Active      bool `json:"active"`
	Hash        string `json:"hash"`
	LastUpdated time.Time `json:"last_updated"`
}

func (repo *Repo) registerOnStatusChange(handler statusChangeHandler) error {
	if repo.seState == nil {
		return errors.New("status endpoint state not correct initialized.")
	}
	return repo.seState.registerOnStatusChange(handler)
}

func (repo *Repo) unregisterOnStatusChange(handler statusChangeHandler) error {
	if repo.seState == nil {
		return errors.New("status endpoint state not correct initialized.")
	}
	return repo.seState.unregisterOnStatusChange(handler)
}

func (repo *Repo) fireStatusChangeEvent() error {
	if repo.seState == nil {
		return nil
	}
	status := repo.toStatus()
	return repo.seState.fireStatusChangeEvent(repo, status)
}

func (repo *Repo) toStatus() Status {
	return Status{
		Active:      repo.pulled,
		Hash:        repo.lastCommit,
		LastUpdated: repo.lastPull,
	}
}
