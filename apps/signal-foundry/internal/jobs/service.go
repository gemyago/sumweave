package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
)

type dispatchPublisher interface {
	PublishInTx(context.Context, *sql.Tx, appdispatch.Envelope) error
}
type ServiceDeps struct {
	Store       *Store
	IDGenerator ident.Generator
	Publisher   dispatchPublisher
	Clock       func() time.Time
	Registry    *Registry
}
type Service struct {
	store       *Store
	idGenerator ident.Generator
	publisher   dispatchPublisher
	clock       func() time.Time
	registry    *Registry
}

func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.IDGenerator == nil {
		return nil, errors.New("id generator is required")
	}
	if deps.Publisher == nil {
		return nil, errors.New("dispatch publisher is required")
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if deps.Registry == nil {
		deps.Registry = NewRegistry()
	}
	return &Service{
		store:       deps.Store,
		idGenerator: deps.IDGenerator,
		publisher:   deps.Publisher,
		clock:       deps.Clock,
		registry:    deps.Registry,
	}, nil
}
func (s *Service) Get(ctx context.Context, jobID string) (*Job, error) {
	job, err := s.store.Get(ctx, jobID)
	if errors.Is(err, ErrJobNotFound) {
		return nil, app.NewErrNotFound("job", strings.TrimSpace(jobID))
	}
	return job, err
}
func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	return s.store.List(ctx, params)
}
func (s *Service) Enqueue(ctx context.Context, params EnqueueParams) (*Job, error) {
	inputJSON, err := EncodeJobPayload(params.Input)
	if err != nil {
		return nil, fmt.Errorf("marshal job input: %w", err)
	}
	return s.EnqueueJSON(
		ctx,
		EnqueueJSONParams{
			JobType:        params.JobType,
			Requester:      params.Requester,
			IdempotencyKey: params.IdempotencyKey,
			CorrelationID:  params.CorrelationID,
			ScheduleID:     params.ScheduleID,
			InputJSON:      inputJSON,
		},
	)
}
func (s *Service) EnqueueJSON(ctx context.Context, params EnqueueJSONParams) (*Job, error) {
	job, _, err := s.enqueueJSONInTx(ctx, nil, params)
	return job, err
}
func (s *Service) enqueueJSONInTx(ctx context.Context, tx *StoreTx, params EnqueueJSONParams) (*Job, bool, error) {
	requester := canonicalizeRequester(params.Requester)
	if requester.Source == "" {
		return nil, false, app.NewErrInvalidInput("requester.source", "requester source is required")
	}
	handler, err := s.registry.Handler(params.JobType)
	if err != nil {
		return nil, false, err
	}
	inputHash := strings.TrimSpace(params.InputHash)
	if len(params.InputJSON) > 0 && inputHash == "" {
		inputHash = hashBytes(params.InputJSON)
	}
	now := s.clock()
	job := Job{
		ID:                 s.idGenerator.MustNewV7().String(),
		JobType:            params.JobType,
		Status:             JobStatusQueued,
		Requester:          requester,
		IdempotencyKey:     strings.TrimSpace(params.IdempotencyKey),
		InputHash:          inputHash,
		InputJSON:          params.InputJSON,
		CreatedAt:          now,
		UpdatedAt:          now,
		QueuedAt:           now,
		MaxAttempts:        handler.maxAttempts(),
		CorrelationID:      strings.TrimSpace(params.CorrelationID),
		ScheduleID:         strings.TrimSpace(params.ScheduleID),
		ScheduledAt:        params.ScheduledAt,
		ScheduledNextRunAt: params.ScheduledNextRunAt,
	}
	var created Job
	createdNew := false
	run := func(storeTx *StoreTx) error {
		if job.IdempotencyKey != "" {
			var createErr error
			created, createdNew, createErr = storeTx.CreateIdempotent(ctx, job)
			if createErr != nil {
				return createErr
			}
		} else {
			var createErr error
			created, createErr = storeTx.Create(ctx, job)
			if createErr != nil {
				return createErr
			}
			createdNew = true
		}
		if !createdNew {
			return nil
		}
		return s.publisher.PublishInTx(
			ctx,
			storeTx.SQLTx(),
			appdispatch.Envelope{
				Version:         appdispatch.EnvelopeVersionV1,
				Kind:            dispatchKindForJobType(created.JobType),
				Payload:         created.InputJSON,
				ObservableJobID: created.ID,
				CorrelationID:   created.CorrelationID,
				RequesterID:     created.Requester.UserID,
				RequesterSource: string(created.Requester.Source),
			},
		)
	}
	if tx != nil {
		err = run(tx)
	} else {
		err = s.store.WithTx(ctx, run)
	}
	if err != nil {
		return nil, false, err
	}
	return &created, createdNew, nil
}
func (s *Service) Cancel(ctx context.Context, jobID string) (*Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return nil, err
	}
	handler, err := s.registry.Handler(job.JobType)
	if err != nil {
		return nil, err
	}
	if !handler.supportsCancel() || job.Status != JobStatusQueued {
		return nil, app.NewErrConflict("job", "job cancel is not supported")
	}
	if err = s.store.MarkCanceled(ctx, job.ID, s.clock()); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, job.ID)
}
func (s *Service) Retry(ctx context.Context, jobID string) (*Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return nil, err
	}
	handler, err := s.registry.Handler(job.JobType)
	if err != nil {
		return nil, err
	}
	if !handler.supportsRetry() || (job.Status != JobStatusFailed && job.Status != JobStatusCanceled) {
		return nil, app.NewErrConflict("job", "job retry is not supported")
	}
	return s.EnqueueJSON(
		ctx,
		EnqueueJSONParams{
			JobType:            job.JobType,
			Requester:          job.Requester,
			CorrelationID:      job.CorrelationID,
			ScheduleID:         job.ScheduleID,
			ScheduledAt:        job.ScheduledAt,
			ScheduledNextRunAt: job.ScheduledNextRunAt,
			InputJSON:          job.InputJSON,
		},
	)
}
