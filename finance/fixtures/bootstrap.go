package fixtures

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

const fixtureSeedMixConstant = 0x9e3779b97f4a7c15

type Config struct {
	Seed               int64
	Now                time.Time
	Scenario           string
	OwnerUserID        string
	MemberUserID       string
	ConnectionProvider string
}

type RunStart struct {
	Seed      int64
	Scenario  string
	StartedAt time.Time
}

type ScenarioRecord struct {
	Name       string
	StableID   string
	OccurredAt time.Time
}

type Summary struct {
	Seed        int64
	Scenario    string
	ScenarioIDs []string
}

type BootstrapService interface {
	StartRun(ctx context.Context, run RunStart) (*RunHandle, error)
}

type scenarioRecorder interface {
	RecordScenario(ctx context.Context, record ScenarioRecord) error
}

type RunHandle struct {
	recorder scenarioRecorder
}

func newRunHandle(recorder scenarioRecorder) *RunHandle {
	return &RunHandle{recorder: recorder}
}

func (h *RunHandle) RecordScenario(ctx context.Context, record ScenarioRecord) error {
	return h.recorder.RecordScenario(ctx, record)
}

type ScenarioBuilder interface {
	Name() string
	Build(ctx ScenarioContext, service *RunHandle) error
}

type ScenarioBuilderFunc func(ctx ScenarioContext, service *RunHandle) error

func (f ScenarioBuilderFunc) Name() string { return "scenario" }

func (f ScenarioBuilderFunc) Build(ctx ScenarioContext, service *RunHandle) error {
	return f(ctx, service)
}

type NamedBuilder struct {
	name    string
	builder ScenarioBuilder
}

func (n NamedBuilder) Name() string { return n.name }

func (n NamedBuilder) Build(ctx ScenarioContext, service *RunHandle) error {
	return n.builder.Build(ctx, service)
}

func NamedScenario(name string, builder ScenarioBuilder) NamedBuilder {
	return NamedBuilder{name: name, builder: builder}
}

type ScenarioContext struct {
	Config Config
	rng    *rand.Rand
}

func (c ScenarioContext) NextStableID(prefix string) string {
	return fmt.Sprintf("%s-%016x", prefix, c.rng.Uint64())
}

type Bootstrapper struct {
	service BootstrapService
}

func NewBootstrapper(service BootstrapService) *Bootstrapper {
	return &Bootstrapper{service: service}
}

func (b *Bootstrapper) Bootstrap(
	ctx context.Context,
	config Config,
	builders ...ScenarioBuilder,
) (Summary, error) {
	normalized := config
	if normalized.Now.IsZero() {
		normalized.Now = time.Now()
	}
	service, err := b.service.StartRun(ctx, RunStart{
		Seed:      normalized.Seed,
		Scenario:  normalized.Scenario,
		StartedAt: normalized.Now,
	})
	if err != nil {
		return Summary{}, err
	}
	//nolint:gosec // Deterministic fixture scaffolding intentionally folds signed seeds into a pseudo-random generator.
	seedA := uint64(normalized.Seed)
	//nolint:gosec // Deterministic fixture scaffolding intentionally folds signed seeds into a pseudo-random generator.
	seedB := uint64(normalized.Seed) ^ fixtureSeedMixConstant
	//nolint:gosec // Deterministic fixture scaffolding intentionally uses a seeded pseudo-random generator.
	scope := ScenarioContext{Config: normalized, rng: rand.New(rand.NewPCG(seedA, seedB))}
	summary := Summary{Seed: normalized.Seed, Scenario: normalized.Scenario}
	for _, builder := range builders {
		buildErr := builder.Build(scope, service)
		if buildErr != nil {
			return Summary{}, buildErr
		}
		summary.ScenarioIDs = append(summary.ScenarioIDs, builder.Name())
	}
	return summary, nil
}
