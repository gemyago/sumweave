package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrBankAuthorizationStale   = errors.New("bank authorization changed")
	ErrBankReauthRequired       = errors.New("bank link needs updating")
	ErrBankAuthorizationInvalid = errors.New("bank account identity is invalid")
)

// BankAuthorization is application bookkeeping stored only in the encrypted
// connection secret. Provider snapshots remain the provider's own documents.
type BankAuthorization struct {
	Version      int                  `json:"version"`
	ASPSPName    string               `json:"aspspName"`
	ASPSPCountry string               `json:"aspspCountry"`
	Accounts     []BankAccountBinding `json:"accounts"`
}

type BankAccountBinding struct {
	CanonicalID string     `json:"canonicalId"`
	PrimaryHash string     `json:"primaryHash"`
	RemoteID    string     `json:"remoteId"`
	Currency    string     `json:"currency"`
	Selected    bool       `json:"selected"`
	ReplayFrom  *time.Time `json:"replayFrom,omitempty"`
}

func (a BankAuthorization) Validate() error {
	if a.Version != 1 || a.ASPSPName == "" || a.ASPSPCountry == "" || len(a.Accounts) == 0 {
		return ErrBankAuthorizationInvalid
	}
	canonical, hashes, remote := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, b := range a.Accounts {
		if b.CanonicalID == "" || b.PrimaryHash == "" || b.RemoteID == "" || b.Currency == "" ||
			canonical[b.CanonicalID] ||
			hashes[b.PrimaryHash] ||
			remote[b.RemoteID] ||
			(b.ReplayFrom != nil && b.ReplayFrom.IsZero()) {
			return fmt.Errorf("%w: missing or duplicate binding", ErrBankAuthorizationInvalid)
		}
		canonical[b.CanonicalID], hashes[b.PrimaryHash], remote[b.RemoteID] = true, true, true
	}
	return nil
}

// Merge retains inactive identities and unfinished replay across replacements.
func (a BankAuthorization) Merge(
	candidate BankAuthorization,
	now, catchupFrom time.Time,
) (BankAuthorization, bool, error) {
	if err := a.Validate(); err != nil {
		return BankAuthorization{}, false, err
	}
	if err := candidate.Validate(); err != nil {
		return BankAuthorization{}, false, err
	}
	if a.ASPSPName != candidate.ASPSPName || a.ASPSPCountry != candidate.ASPSPCountry {
		return BankAuthorization{}, false, fmt.Errorf("%w: bank changed", ErrBankAuthorizationInvalid)
	}
	merged := a
	merged.Accounts = append([]BankAccountBinding(nil), a.Accounts...)
	byHash, byCanonical := map[string]int{}, map[string]bool{}
	for i, b := range merged.Accounts {
		byHash[b.PrimaryHash], byCanonical[b.CanonicalID] = i, true
		if b.Selected {
			merged.Accounts[i].ReplayFrom = earlierReplay(b.ReplayFrom, catchupFrom)
		}
		merged.Accounts[i].Selected = false
	}
	replay := false
	for _, b := range candidate.Accounts {
		if i, exists := byHash[b.PrimaryHash]; exists {
			old := a.Accounts[i]
			if old.Currency != b.Currency {
				return BankAuthorization{}, false, fmt.Errorf("%w: currency changed", ErrBankAuthorizationInvalid)
			}
			merged.Accounts[i].RemoteID, merged.Accounts[i].Selected = b.RemoteID, true
			merged.Accounts[i].ReplayFrom = old.ReplayFrom
			if !old.Selected {
				merged.Accounts[i].ReplayFrom = earlierReplay(old.ReplayFrom, catchupFrom)
				replay = true
			}
		} else {
			if byCanonical[b.RemoteID] {
				return BankAuthorization{}, false, fmt.Errorf("%w: canonical key reused", ErrBankAuthorizationInvalid)
			}
			b.CanonicalID, b.Selected = b.RemoteID, true
			floor := now.AddDate(-3, 0, 0)
			b.ReplayFrom = &floor
			merged.Accounts = append(merged.Accounts, b)
			byCanonical[b.CanonicalID] = true
			replay = true
		}
	}
	if err := merged.Validate(); err != nil {
		return BankAuthorization{}, false, err
	}
	return merged, replay, nil
}

func earlierReplay(existing *time.Time, value time.Time) *time.Time {
	if existing != nil && existing.Before(value) {
		value = *existing
	}
	return &value
}

func (a BankAuthorization) ReplayStart() *time.Time {
	var start *time.Time
	for _, b := range a.Accounts {
		if b.Selected && b.ReplayFrom != nil {
			start = earlierReplay(start, *b.ReplayFrom)
		}
	}
	return start
}
