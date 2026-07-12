package fixtures

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapper(t *testing.T) {
	t.Run(
		"uses deterministic seed and reusable builders through a service-backed path",
		func(t *testing.T) {
			fake := faker.New()
			config := Config{
				Seed: 4242,
				Now: time.Date(
					2026,
					time.June,
					20,
					15,
					0,
					0,
					0,
					time.FixedZone("fixture", 3*60*60),
				),
				Scenario: fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
			}

			first := &recordingBootstrapService{}
			second := &recordingBootstrapService{}

			builder := ScenarioBuilderFunc(func(ctx ScenarioContext, handle *RunHandle) error {
				return handle.RecordScenario(t.Context(), ScenarioRecord{
					Name:       ctx.Config.Scenario,
					StableID:   ctx.NextStableID("tenant"),
					OccurredAt: ctx.Config.Now,
				})
			})

			bootstrapper := NewBootstrapper(first)
			summaryA, err := bootstrapper.Bootstrap(
				t.Context(),
				config,
				NamedScenario(config.Scenario, builder),
			)
			require.NoError(t, err)

			bootstrapper = NewBootstrapper(second)
			summaryB, err := bootstrapper.Bootstrap(
				t.Context(),
				config,
				NamedScenario(config.Scenario, builder),
			)
			require.NoError(t, err)

			require.Len(t, first.records, 1)
			require.Len(t, second.records, 1)
			assert.Equal(t, first.records[0].StableID, second.records[0].StableID)
			assert.Equal(t, config.Now, first.records[0].OccurredAt)
			assert.Equal(t, summaryA, summaryB)
		},
	)

	t.Run("normalizes zero-time runs and surfaces service or builder errors", func(t *testing.T) {
		fake := faker.New()
		config := Config{Seed: 9, Scenario: fmt.Sprintf("scenario-%s", fake.Lorem().Word())}

		service := &recordingBootstrapService{}
		bootstrapper := NewBootstrapper(service)
		summary, err := bootstrapper.Bootstrap(
			t.Context(),
			config,
			NamedScenario(
				config.Scenario,
				ScenarioBuilderFunc(func(ctx ScenarioContext, handle *RunHandle) error {
					return handle.RecordScenario(t.Context(), ScenarioRecord{
						Name:     ctx.Config.Scenario,
						StableID: ctx.NextStableID("seeded"),
					})
				}),
			),
		)
		require.NoError(t, err)
		assert.Equal(t, config.Scenario, summary.Scenario)
		require.Len(t, service.starts, 1)
		assert.False(t, service.starts[0].StartedAt.IsZero())

		bootstrapper = NewBootstrapper(&failingBootstrapService{err: assert.AnError})
		_, err = bootstrapper.Bootstrap(t.Context(), config)
		require.ErrorIs(t, err, assert.AnError)

		bootstrapper = NewBootstrapper(service)
		_, err = bootstrapper.Bootstrap(
			t.Context(),
			config,
			NamedScenario(
				config.Scenario,
				ScenarioBuilderFunc(func(ScenarioContext, *RunHandle) error {
					return assert.AnError
				}),
			),
		)
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestService(t *testing.T) {
	t.Run("records bootstrap runs via repository seams", func(t *testing.T) {
		fake := faker.New()
		repo := &recordingRepository{}
		service := NewService(repo)

		handle, err := service.StartRun(t.Context(), RunStart{
			Seed:     77,
			Scenario: fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
			StartedAt: time.Date(
				2026,
				time.June,
				20,
				16,
				0,
				0,
				0,
				time.FixedZone("fixture", -4*60*60),
			),
		})
		require.NoError(t, err)

		err = handle.RecordScenario(t.Context(), ScenarioRecord{
			Name:     fmt.Sprintf("record-%s", fake.Lorem().Word()),
			StableID: fmt.Sprintf("stable-%s", fake.Lorem().Word()),
			OccurredAt: time.Date(
				2026,
				time.June,
				20,
				16,
				5,
				0,
				0,
				time.FixedZone("fixture", -4*60*60),
			),
		})
		require.NoError(t, err)

		require.Len(t, repo.runs, 1)
		require.Len(t, repo.records, 1)
		assert.Equal(t, time.FixedZone("fixture", -4*60*60), repo.runs[0].StartedAt.Location())
		assert.Equal(t, time.FixedZone("fixture", -4*60*60), repo.records[0].OccurredAt.Location())
	})

	t.Run(
		"propagates repository failures and persistence repository maps to domain store",
		func(t *testing.T) {
			fake := faker.New()
			service := NewService(&failingRepository{err: assert.AnError})

			_, err := service.StartRun(t.Context(), RunStart{
				Seed:      88,
				Scenario:  fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
				StartedAt: time.Date(2026, time.June, 20, 17, 0, 0, 0, time.UTC),
			})
			require.ErrorIs(t, err, assert.AnError)

			store := &recordingFixtureStore{}
			repo := NewPersistenceRepository(store)
			require.NoError(t, repo.CreateRun(t.Context(), RunStart{
				Seed:     99,
				Scenario: fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
				StartedAt: time.Date(
					2026,
					time.June,
					20,
					17,
					5,
					0,
					0,
					time.FixedZone("fixture", 2*60*60),
				),
			}))
			require.NoError(t, repo.CreateScenarioRecord(t.Context(), "run-1", ScenarioRecord{
				Name:     fmt.Sprintf("record-%s", fake.Lorem().Word()),
				StableID: fmt.Sprintf("stable-%s", fake.Lorem().Word()),
				OccurredAt: time.Date(
					2026,
					time.June,
					20,
					17,
					6,
					0,
					0,
					time.FixedZone("fixture", 2*60*60),
				),
			}))
			require.Len(t, store.runs, 1)
			require.Len(t, store.records, 1)
			assert.Equal(t, time.FixedZone("fixture", 2*60*60), store.runs[0].StartedAt.Location())
			assert.Equal(t, time.FixedZone("fixture", 2*60*60), store.records[0].OccurredAt.Location())
		},
	)
}

type recordingBootstrapService struct {
	starts  []RunStart
	records []ScenarioRecord
}

func (s *recordingBootstrapService) StartRun(_ context.Context, run RunStart) (*RunHandle, error) {
	s.starts = append(s.starts, run)
	return newRunHandle(s), nil
}

func (s *recordingBootstrapService) RecordScenario(_ context.Context, record ScenarioRecord) error {
	s.records = append(s.records, record)
	return nil
}

type recordingRepository struct {
	runs    []RunStart
	records []ScenarioRecord
}

func (r *recordingRepository) CreateRun(_ context.Context, run RunStart) error {
	r.runs = append(r.runs, run)
	return nil
}

func (r *recordingRepository) CreateScenarioRecord(
	_ context.Context,
	_ string,
	record ScenarioRecord,
) error {
	r.records = append(r.records, record)
	return nil
}

type failingBootstrapService struct {
	err error
}

func (s *failingBootstrapService) StartRun(context.Context, RunStart) (*RunHandle, error) {
	return nil, s.err
}

type failingRepository struct {
	err error
}

func (r *failingRepository) CreateRun(context.Context, RunStart) error {
	return r.err
}

func (r *failingRepository) CreateScenarioRecord(context.Context, string, ScenarioRecord) error {
	return r.err
}

type recordingFixtureStore struct {
	runs    []domain.FixtureBootstrapRun
	records []domain.FixtureScenarioRecord
}

func (s *recordingFixtureStore) CreateFixtureBootstrapRun(
	_ context.Context,
	run domain.FixtureBootstrapRun,
) error {
	s.runs = append(s.runs, run)
	return nil
}

func (s *recordingFixtureStore) CreateFixtureScenarioRecord(
	_ context.Context,
	_ string,
	record domain.FixtureScenarioRecord,
) error {
	s.records = append(s.records, record)
	return nil
}
