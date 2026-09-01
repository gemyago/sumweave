package finance

import (
	"math"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSemanticCommands(t *testing.T) {
	fake := faker.New()

	t.Run("publishes the direct FX refresh command and returns its future reference", func(t *testing.T) {
		providerName := "provider-" + fake.Letter()
		publisher := NewMockSemanticCommandPublisher(t)
		expectedReference := DispatchReference{MessageID: fake.UUID().V4()}
		publisher.EXPECT().PublishSemanticCommand(
			mock.Anything,
			mock.MatchedBy(func(command SemanticCommand) bool {
				return command.Topic == FXRatesRefreshCommandTopic && command.IdempotencyKey == ""
			}),
		).Return(expectedReference, nil)
		service := NewFXService(
			nil,
			WithFXServiceProviders(NewStaticFXProvider(providerName, nil)),
			WithFXServiceCommandPublisher(publisher),
		)

		reference, err := service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{
			RequestedByUserID: "user-" + fake.UUID().V4(),
			Source:            CommandRequesterSourceOperator,
			Provider:          providerName,
		})

		require.NoError(t, err)
		require.Equal(t, expectedReference.MessageID, reference.ID)
	})

	t.Run("does not publish an unknown FX provider", func(t *testing.T) {
		providerName := "provider-" + fake.Letter()
		service := NewFXService(
			nil,
			WithFXServiceProviders(NewStaticFXProvider(providerName, nil)),
			WithFXServiceCommandPublisher(NewMockSemanticCommandPublisher(t)),
		)

		_, err := service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{Provider: "missing-" + fake.Letter()})

		require.Error(t, err)
	})

	t.Run("returns publication errors without inline refresh work", func(t *testing.T) {
		providerName := "provider-" + fake.Letter()
		publisher := NewMockSemanticCommandPublisher(t)
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.Anything).Return(
			DispatchReference{},
			assert.AnError,
		)
		service := NewFXService(
			nil,
			WithFXServiceProviders(NewStaticFXProvider(providerName, nil)),
			WithFXServiceCommandPublisher(publisher),
		)

		_, err := service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{Provider: providerName})

		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("reports commands that cannot be safely serialized", func(t *testing.T) {
		_, err := newSemanticCommand(FXRatesRefreshCommandTopic, math.Inf(1), "")

		require.Error(t, err)
	})
}
