package fixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/gemyago/sumweave/finance/domain"
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
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	return newRunHandle(&serviceRunHandle{repo: s.repo, runID: makeRunID(run)}), nil
}

type serviceRunHandle struct {
	repo  Repository
	runID string
}

func (h *serviceRunHandle) RecordScenario(ctx context.Context, record ScenarioRecord) error {
	return h.repo.CreateScenarioRecord(ctx, h.runID, record)
}

func makeRunID(run RunStart) string {
	stableInput := run.Scenario + strconv.FormatInt(run.StartedAt.UnixNano(), 10) + strconv.FormatInt(run.Seed, 10)
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
		StartedAt: run.StartedAt,
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
		OccurredAt: record.OccurredAt,
	})
}
