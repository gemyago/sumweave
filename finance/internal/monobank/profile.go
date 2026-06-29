package monobank

import (
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
)

func Profile() providers.ProviderProfile {
	return providers.ProviderProfile{
		ProviderID:    domain.ProviderIDMonobank,
		ConnectorID:   domain.ProviderConnectorIDMonobank,
		DisplayName:   "Monobank",
		CountryCode:   "UA",
		MarketSegment: "personal",
	}
}
