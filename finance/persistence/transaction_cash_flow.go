package persistence

import (
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
)

func filterTransactionCashFlow(query *gorm.DB, inclusion domain.CashFlowInclusion) *gorm.DB {
	if inclusion == "" || inclusion == domain.CashFlowInclusionBoth {
		return query
	}
	groups := []string{"neutral"}
	if inclusion == domain.CashFlowInclusionIncome || inclusion == domain.CashFlowInclusionExpense {
		groups = append(groups, string(inclusion))
	}
	// Mirror reportingContribution, but leave hidden/status visibility to the
	// caller's independent filters. A refund reduces expense even when positive.
	return query.Where(`(CASE
		WHEN kind = ? THEN 'expense'
		WHEN kind = ? AND status = ? AND transfer_matched_at IS NOT NULL
			AND transfer_matched_at <> ? THEN 'neutral'
		WHEN kind IN ? AND amount_minor > 0 THEN 'income'
		WHEN kind IN ? AND amount_minor < 0 THEN 'expense'
		ELSE 'neutral'
	END) IN ?`, domain.TransactionKindRefund, domain.TransactionKindTransfer,
		domain.TransactionStatusBooked, time.Time{},
		[]domain.TransactionKind{domain.TransactionKindRegular, domain.TransactionKindIncome,
			domain.TransactionKindExpense, domain.TransactionKindTransfer},
		[]domain.TransactionKind{domain.TransactionKindRegular, domain.TransactionKindIncome,
			domain.TransactionKindExpense, domain.TransactionKindTransfer}, groups)
}
