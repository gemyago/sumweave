package telemetry

import (
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

func Test_detectEndpointSecurity(t *testing.T) {
	fake := faker.New()

	t.Run("detects secure endpoint", func(t *testing.T) {
		expectedEndpoint := fake.Internet().Domain()
		endpoint := "https://" + expectedEndpoint

		actualEndpoint, isSecure := detectEndpointSecurity(endpoint)

		assert.Equal(t, expectedEndpoint, actualEndpoint)
		assert.True(t, isSecure)
	})

	t.Run("detects insecure endpoint", func(t *testing.T) {
		expectedEndpoint := fake.Internet().Domain()
		endpoint := "http://" + expectedEndpoint

		actualEndpoint, isSecure := detectEndpointSecurity(endpoint)

		assert.Equal(t, expectedEndpoint, actualEndpoint)
		assert.False(t, isSecure)
	})

	t.Run("detects endpoint without scheme as insecure", func(t *testing.T) {
		expectedEndpoint := fake.Internet().Domain()
		endpoint := expectedEndpoint

		actualEndpoint, isSecure := detectEndpointSecurity(endpoint)

		assert.Equal(t, expectedEndpoint, actualEndpoint)
		assert.False(t, isSecure)
	})
}

func TestNewResource(t *testing.T) {
	fake := faker.New()
	environment := fake.Lorem().Word()

	resource, err := NewResource(t.Context(), ResourceDeps{Environment: environment})

	require.NoError(t, err)
	value, ok := resource.Set().Value(semconv.DeploymentEnvironmentNameKey)
	require.True(t, ok)
	assert.Equal(t, environment, value.AsString())
}
