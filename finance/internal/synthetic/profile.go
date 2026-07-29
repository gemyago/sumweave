package synthetic

import (
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
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
