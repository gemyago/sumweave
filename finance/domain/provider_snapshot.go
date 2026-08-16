package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ProviderSnapshotSubject identifies the finance object exposing source data.
type ProviderSnapshotSubject string

const (
	ProviderSnapshotSubjectConnection  ProviderSnapshotSubject = "connection"
	ProviderSnapshotSubjectAccount     ProviderSnapshotSubject = "account"
	ProviderSnapshotSubjectTransaction ProviderSnapshotSubject = "transaction"
)

// ProviderSnapshotKind identifies the schema-derived provider document shape.
type ProviderSnapshotKind string

const (
	ProviderSnapshotKindConnection     ProviderSnapshotKind = "connection"
	ProviderSnapshotKindAccount        ProviderSnapshotKind = "account"
	ProviderSnapshotKindAccountBalance ProviderSnapshotKind = "account_balance"
	ProviderSnapshotKindTransaction    ProviderSnapshotKind = "transaction"
)

// ProviderSnapshot is the current sanitized source document for one provider object.
type ProviderSnapshot struct {
	ID                   string
	TenantID             string
	ConnectionID         string
	FinanceAccountID     string
	FinanceTransactionID string
	Subject              ProviderSnapshotSubject
	Kind                 ProviderSnapshotKind
	ProviderObjectID     string
	DocumentJSON         []byte
	CapturedAt           time.Time
}

// Validate confirms that a source snapshot has an unambiguous finance attachment.
func (s ProviderSnapshot) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "snapshot id", value: s.ID},
		{name: "tenant id", value: s.TenantID},
		{name: "connection id", value: s.ConnectionID},
		{name: "provider object id", value: s.ProviderObjectID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New(field.name + " is required")
		}
	}
	if !supportedProviderSnapshotSubject(s.Subject) {
		return errors.New("provider snapshot subject is unsupported")
	}
	if !supportedProviderSnapshotKind(s.Kind) {
		return errors.New("provider snapshot kind is unsupported")
	}
	if !json.Valid(s.DocumentJSON) {
		return errors.New("provider snapshot document must be valid JSON")
	}
	if s.CapturedAt.IsZero() {
		return errors.New("provider snapshot capture time is required")
	}
	return s.validateAttachment()
}

func (s ProviderSnapshot) validateAttachment() error {
	switch s.Subject {
	case ProviderSnapshotSubjectConnection:
		if s.FinanceAccountID != "" || s.FinanceTransactionID != "" {
			return errors.New("connection snapshot must not identify a finance account or transaction")
		}
		if s.Kind != ProviderSnapshotKindConnection {
			return errors.New("connection snapshot kind must be connection")
		}
	case ProviderSnapshotSubjectAccount:
		if strings.TrimSpace(s.FinanceAccountID) == "" {
			return errors.New("account snapshot finance account id is required")
		}
		if s.FinanceTransactionID != "" {
			return errors.New("account snapshot must not identify a finance transaction")
		}
		if s.Kind != ProviderSnapshotKindAccount && s.Kind != ProviderSnapshotKindAccountBalance {
			return errors.New("account snapshot kind must be account or account_balance")
		}
	case ProviderSnapshotSubjectTransaction:
		if strings.TrimSpace(s.FinanceAccountID) == "" {
			return errors.New("transaction snapshot finance account id is required")
		}
		if strings.TrimSpace(s.FinanceTransactionID) == "" {
			return errors.New("transaction snapshot finance transaction id is required")
		}
		if s.Kind != ProviderSnapshotKindTransaction {
			return errors.New("transaction snapshot kind must be transaction")
		}
	}
	return nil
}

func supportedProviderSnapshotSubject(subject ProviderSnapshotSubject) bool {
	switch subject {
	case ProviderSnapshotSubjectConnection, ProviderSnapshotSubjectAccount, ProviderSnapshotSubjectTransaction:
		return true
	default:
		return false
	}
}

func supportedProviderSnapshotKind(kind ProviderSnapshotKind) bool {
	switch kind {
	case ProviderSnapshotKindConnection,
		ProviderSnapshotKindAccount,
		ProviderSnapshotKindAccountBalance,
		ProviderSnapshotKindTransaction:
		return true
	default:
		return false
	}
}

// SanitizeProviderSnapshotJSON removes credential-like values from a source document.
func SanitizeProviderSnapshotJSON(document []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(document, &value); err != nil {
		return nil, err
	}
	return json.Marshal(sanitizeProviderSnapshotValue(value))
}

func sanitizeProviderSnapshotValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, item := range typed {
			if credentialLikeProviderSnapshotKey(key) {
				continue
			}
			sanitized[key] = sanitizeProviderSnapshotValue(item)
		}
		return sanitized
	case []any:
		sanitized := make([]any, 0, len(typed))
		for _, item := range typed {
			sanitized = append(sanitized, sanitizeProviderSnapshotValue(item))
		}
		return sanitized
	default:
		return value
	}
}

func credentialLikeProviderSnapshotKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		normalized == "authorization" ||
		strings.Contains(normalized, "bearer") ||
		strings.Contains(normalized, "privatekey") ||
		strings.Contains(normalized, "jwt") ||
		strings.Contains(normalized, "signature") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "accesskey") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "passphrase")
}
