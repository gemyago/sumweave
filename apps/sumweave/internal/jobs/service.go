package jobs

import (
	"context"
	"errors"
	"strings"

	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
)

// ServiceDeps configures the jobs read model. Commands are published directly to
// appdispatch, and lifecycle writes are performed by observed consumers.
type ServiceDeps struct{ Store *Store }

type Service struct{ store *Store }

func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil { // coverage-ignore // Constructor dependency failure is exercised by wireup callers.
		return nil, errors.New("jobs store is required")
	}
	return &Service{store: deps.Store}, nil
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
