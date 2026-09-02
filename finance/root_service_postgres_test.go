//go:build postgres_test

package finance

import "context"

func (s *Service) loadDashboardData(
	ctx context.Context,
	tenantID string,
	params DashboardParams,
) (dashboardData, error) {
	return s.reporting.loadDashboardData(ctx, tenantID, params)
}
