package monobank

import (
	"testing"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/stretchr/testify/assert"
)

func TestProfile(t *testing.T) {
	assert.Equal(t, domain.ProviderIDMonobank, Profile().ProviderID)
	assert.Equal(t, domain.ProviderConnectorIDMonobank, Profile().ConnectorID)
	assert.Equal(t, "Monobank", Profile().DisplayName)
	assert.Equal(t, "UA", Profile().CountryCode)
	assert.Equal(t, "personal", Profile().MarketSegment)
}
