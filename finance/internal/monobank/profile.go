package monobank

import (
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
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
