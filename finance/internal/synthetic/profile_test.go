package synthetic

import (
	"testing"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/stretchr/testify/assert"
)

func TestProfile(t *testing.T) {
	assert.Equal(t, domain.ProviderIDSynthetic, Profile().ProviderID)
	assert.Equal(t, domain.ProviderConnectorIDSynthetic, Profile().ConnectorID)
	assert.Equal(t, "Synthetic", Profile().DisplayName)
	assert.Equal(t, "ZZ", Profile().CountryCode)
	assert.Equal(t, "test", Profile().MarketSegment)
}
