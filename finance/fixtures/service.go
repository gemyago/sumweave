package fixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

type Repository interface {
	CreateRun(ctx context.Context, run RunStart) error
	CreateScenarioRecord(ctx context.Context, runID string, record ScenarioRecord) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) StartRun(ctx context.Context, run RunStart) (*RunHandle, error) {
	normalized := run
	normalized.StartedAt = normalized.StartedAt.UTC()
	if err := s.repo.CreateRun(ctx, normalized); err != nil {
		return nil, err
	}
	return newRunHandle(&serviceRunHandle{repo: s.repo, runID: makeRunID(normalized)}), nil
}

type serviceRunHandle struct {
	repo  Repository
	runID string
}

func (h *serviceRunHandle) RecordScenario(ctx context.Context, record ScenarioRecord) error {
	normalized := record
	normalized.OccurredAt = normalized.OccurredAt.UTC()
	return h.repo.CreateScenarioRecord(ctx, h.runID, normalized)
}

func makeRunID(run RunStart) string {
	stableInput := run.Scenario +
		run.StartedAt.UTC().Format(time.RFC3339Nano) +
		strconv.FormatInt(run.Seed, 10)
	hash := sha256.Sum256([]byte(stableInput))
	return hex.EncodeToString(hash[:8])
}

type fixtureStore interface {
	CreateFixtureBootstrapRun(ctx context.Context, run domain.FixtureBootstrapRun) error
	CreateFixtureScenarioRecord(
		ctx context.Context,
		runID string,
		record domain.FixtureScenarioRecord,
	) error
}

type PersistenceRepository struct {
	store fixtureStore
}

func NewPersistenceRepository(store fixtureStore) *PersistenceRepository {
	return &PersistenceRepository{store: store}
}

func (r *PersistenceRepository) CreateRun(ctx context.Context, run RunStart) error {
	return r.store.CreateFixtureBootstrapRun(ctx, domain.FixtureBootstrapRun{
		ID:        makeRunID(run),
		Seed:      run.Seed,
		Scenario:  run.Scenario,
		StartedAt: run.StartedAt.UTC(),
	})
}

func (r *PersistenceRepository) CreateScenarioRecord(
	ctx context.Context,
	runID string,
	record ScenarioRecord,
) error {
	return r.store.CreateFixtureScenarioRecord(ctx, runID, domain.FixtureScenarioRecord{
		Name:       record.Name,
		StableID:   record.StableID,
		OccurredAt: record.OccurredAt.UTC(),
	})
}
