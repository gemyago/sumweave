package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
)

type createWakeSignaler interface {
	SignalWake(jobID string)
}

type ServiceDeps struct {
	Store       *Store
	IDGenerator ident.Generator
	Clock       func() time.Time
	Limits      HistoricalBackfillLimits
	Wake        createWakeSignaler
}

type Service struct {
	store       *Store
	idGenerator ident.Generator
	clock       func() time.Time
	limits      HistoricalBackfillLimits
	wake        createWakeSignaler
}

func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.IDGenerator == nil {
		return nil, errors.New("id generator is required")
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	return &Service{
		store:       deps.Store,
		idGenerator: deps.IDGenerator,
		clock:       deps.Clock,
		limits:      normalizeHistoricalBackfillLimits(deps.Limits),
		wake:        deps.Wake,
	}, nil
}

func (s *Service) Get(ctx context.Context, jobID string) (*Job, error) {
	job, err := s.store.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, app.NewErrNotFound("job", strings.TrimSpace(jobID))
		}
		return nil, err
	}
	return job, nil
}

func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	return s.store.List(ctx, params)
}

func (s *Service) CreateHistoricalRawCandleBackfill(
	ctx context.Context,
	params CreateHistoricalRawCandleBackfillParams,
) (*Job, error) {
	requester := canonicalizeRequester(params.Requester)
	if requester.Source == "" {
		return nil, app.NewErrInvalidInput("requester.source", "requester source is required")
	}
	input, err := validateHistoricalBackfillInput(HistoricalRawCandleBackfillInput{
		IngestionRunID: s.idGenerator.MustNewV7().String(),
		Venue:          params.Venue,
		Symbol:         params.Symbol,
		AssetClass:     params.AssetClass,
		Timeframe:      params.Timeframe,
		Start:          params.Start,
		End:            params.End,
		PageSize:       params.PageSize,
	}, s.limits, s.clock())
	if err != nil {
		return nil, app.NewErrInvalidInput("request", err.Error())
	}
	inputHash, err := HashInput(input)
	if err != nil {
		return nil, fmt.Errorf("hash job input: %w", err)
	}
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	now := s.clock().UTC()
	jobID := s.idGenerator.MustNewV7().String()
	job := Job{
		ID:             jobID,
		JobType:        JobTypeHistoricalRawCandleBackfill,
		Status:         JobStatusQueued,
		Requester:      requester,
		IdempotencyKey: idempotencyKey,
		InputHash:      inputHash,
		Input:          input,
		CreatedAt:      now,
		UpdatedAt:      now,
		QueuedAt:       now,
		AttemptCount:   0,
		CorrelationID:  strings.TrimSpace(params.CorrelationID),
	}
	if idempotencyKey != "" {
		created, createdNew, createErr := s.store.CreateIdempotent(ctx, job)
		if createErr != nil {
			return nil, createErr
		}
		if createdNew && s.wake != nil {
			s.wake.SignalWake(created.ID)
		}
		return &created, nil
	}
	created, err := s.store.Create(ctx, job)
	if err != nil {
		return nil, err
	}
	if s.wake != nil {
		s.wake.SignalWake(created.ID)
	}
	return &created, nil
}
