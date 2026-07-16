package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderEvidenceSubject(t *testing.T) {
	t.Run("keeps entity and connection subjects distinct while reserving empty for legacy rows", func(t *testing.T) {
		assert.Equal(t, ProviderEvidenceSubjectAccount, ProviderEvidenceSubject("account"))
		assert.Equal(t, ProviderEvidenceSubjectTransaction, ProviderEvidenceSubject("transaction"))
		assert.Equal(t, ProviderEvidenceSubjectConnection, ProviderEvidenceSubject("connection"))
		assert.Empty(t, ProviderEvidence{}.Subject)
	})
}
