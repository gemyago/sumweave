package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	TransactionCSVImportCommandTopic = "finance.csv-import.transactions.v1"
	AccountCSVImportCommandTopic     = "finance.csv-import.accounts.v1"
	BankConnectionSyncCommandTopic   = "finance.bank-connection-sync.v1"
	FXRatesRefreshCommandTopic       = "finance.fx-rates-refresh.v1"
	CommandRequesterSourceOperator   = "operator"
	CommandRequesterSourceSystem     = "system"
)

// SemanticCommandPublisher publishes finance-owned commands without coupling
// finance services to a particular durable transport or job projection.
type SemanticCommandPublisher interface {
	PublishSemanticCommand(context.Context, SemanticCommand) (DispatchReference, error)
}

// SemanticCommand is a serialized finance command ready for durable
// publication. The topic and payload contracts are defined in this package.
type SemanticCommand struct {
	Topic          string
	Payload        []byte
	IdempotencyKey string
}

// DispatchReference identifies a future delivery of a semantic command.
type DispatchReference struct {
	MessageID string
}

// CommandRequester carries only the job-projection metadata required by an
// observed consumer. It deliberately contains no credential or provider data.
type CommandRequester struct {
	UserID string `json:"userId"`
	Source string `json:"source"`
}

// CSVImportCommand is the safe handler input for either CSV import workload.
type CSVImportCommand struct {
	ImportID  string           `json:"importId"`
	Requester CommandRequester `json:"requester"`
}

// BankConnectionSyncCommand is the safe handler input for one bank sync.
// Scheduled timestamps are present only for a scheduled occurrence.
type BankConnectionSyncCommand struct {
	ConnectionID       string           `json:"connectionId"`
	Reason             string           `json:"reason"`
	WindowStart        *time.Time       `json:"windowStart,omitempty"`
	WindowEnd          *time.Time       `json:"windowEnd,omitempty"`
	Requester          CommandRequester `json:"requester"`
	ScheduledAt        *time.Time       `json:"scheduledAt,omitempty"`
	ScheduledNextRunAt *time.Time       `json:"scheduledNextRunAt,omitempty"`
}

// FXRatesRefreshCommand is the safe handler input for one FX refresh.
type FXRatesRefreshCommand struct {
	Provider  string           `json:"provider"`
	Requester CommandRequester `json:"requester"`
}

func newSemanticCommand(topic string, payload any, idempotencyKey string) (SemanticCommand, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SemanticCommand{}, fmt.Errorf("marshal finance semantic command: %w", err)
	}
	return SemanticCommand{Topic: topic, Payload: encoded, IdempotencyKey: idempotencyKey}, nil
}
