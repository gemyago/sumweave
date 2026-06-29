package synthetic

import (
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
)

func Profile() providers.ProviderProfile {
	return providers.ProviderProfile{
		ProviderID:    domain.ProviderIDSynthetic,
		ConnectorID:   domain.ProviderConnectorIDSynthetic,
		DisplayName:   "Synthetic",
		CountryCode:   "ZZ",
		MarketSegment: "test",
	}
}
