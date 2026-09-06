package finance

import (
	"context"
	"database/sql"

	"github.com/gemyago/sumweave/finance/domain"
)

// BankSyncWindowCompletionPublisher publishes a committed requested-window fact
// through the caller's active SQL transaction.
type BankSyncWindowCompletionPublisher interface {
	PublishBankSyncWindowCompleted(context.Context, *sql.Tx, domain.BankSyncWindowCompleted) error
}
