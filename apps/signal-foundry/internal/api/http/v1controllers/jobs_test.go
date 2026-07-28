package v1controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJobsController(t *testing.T) {
	fake := faker.New()
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(
				w,
				r.WithContext(
					httpapi.ContextWithCallerIdentity(
						r.Context(),
						&testCallerIdentity{userID: fake.UUID().V4()},
					),
				),
			)
		})
	}
	now := time.Now()
	job := jobspkg.Job{
		ID:      fake.UUID().V4(),
		JobType: jobspkg.JobType("finance.csv_import"),
		Status:  jobspkg.JobStatusSucceeded,
		Requester: jobspkg.Requester{
			UserID: fake.UUID().V4(),
			Source: jobspkg.RequesterSourceOperator,
		},
		CreatedAt:     now,
		UpdatedAt:     now,
		StartedAt:     &now,
		CompletedAt:   &now,
		LastAttemptAt: &now,
		AttemptCount:  1,
		Error: &jobspkg.JobError{
			Code:    fake.UUID().V4(),
			Summary: fake.Lorem().Sentence(3),
			Details: fake.Lorem().Sentence(4),
		},
	}
	t.Run(
		"registered list and detail routes require a caller and expose metadata only",
		func(t *testing.T) {
			service := newMockjobsService(t)
			service.EXPECT().
				List(mock.Anything, mock.Anything).
				Return(jobspkg.ListResult{Items: []jobspkg.Job{job}, NextCursor: fake.UUID().V4()}, nil)
			service.EXPECT().Get(mock.Anything, job.ID).Return(&job, nil)
			handler := server.NewTestRootHandler().
				RegisterJobsRoutes(NewJobsController(JobsControllerDeps{JobsService: service, AuthMiddleware: middleware.AuthMiddleware(auth)}))
			for _, target := range []string{"/api/v1/jobs?status=queued&jobType=finance.csv_import&source=operator&limit=1", "/api/v1/jobs/" + job.ID} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
				require.Equal(t, http.StatusOK, response.Code)
				require.NotContains(t, response.Body.String(), "input")
			}
		},
	)
	t.Run("mapping helpers discard empty filters and preserve optional fields", func(t *testing.T) {
		require.Empty(t, mapJobStatuses([]string{"", " "}))
		require.Empty(t, mapJobTypes([]string{" "}))
		require.Empty(t, mapRequesterSources([]string{""}))
		require.NotNil(t, mapJobSummary(job).Error)
		require.NotNil(t, mapJobDetail(job).Requester)
		require.Error(t, requireOperatorRequester(t.Context()))
	})
}
