package v1controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJobsController(t *testing.T) {
	fake := faker.New()
	userID := fake.UUID().V4()
	makeAuth := func(caller auth.Caller) middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(auth.ContextWithCaller(r.Context(), caller)))
			})
		}
	}
	now := time.Now()
	job := jobspkg.Job{
		ID:      fake.UUID().V4(),
		JobType: jobspkg.JobType("finance.csv_import"),
		Status:  jobspkg.JobStatusSucceeded,
		Requester: jobspkg.Requester{
			UserID: userID,
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
		"registered list and detail routes apply session scope and expose metadata only",
		func(t *testing.T) {
			service := newMockjobsService(t)
			service.EXPECT().
				List(mock.Anything, jobspkg.ListParams{
					RequesterUserID: userID,
					AllowedSources: []jobspkg.RequesterSource{
						jobspkg.RequesterSourceOperator,
						jobspkg.RequesterSourceIntegration,
					},
					Statuses: []jobspkg.JobStatus{jobspkg.JobStatusQueued},
					JobTypes: []jobspkg.JobType{"finance.csv_import"},
					Sources:  []jobspkg.RequesterSource{jobspkg.RequesterSourceOperator},
					Limit:    1,
				}).
				Return(jobspkg.ListResult{Items: []jobspkg.Job{job}, NextCursor: fake.UUID().V4()}, nil)
			service.EXPECT().Get(mock.Anything, jobspkg.GetParams{
				JobID:           job.ID,
				RequesterUserID: userID,
				AllowedSources: []jobspkg.RequesterSource{
					jobspkg.RequesterSourceOperator,
					jobspkg.RequesterSourceIntegration,
				},
			}).Return(&job, nil)
			auth := makeAuth(auth.Caller{UserID: userID, Credential: auth.CredentialKindSession})
			handler := server.NewTestRootHandler().
				RegisterJobsRoutes(NewJobsController(JobsControllerDeps{JobsService: service, TokenReadMiddleware: auth}))
			for _, target := range []string{"/api/v1/jobs?status=queued&jobType=finance.csv_import&source=operator&limit=1", "/api/v1/jobs/" + job.ID} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
				require.Equal(t, http.StatusOK, response.Code)
				require.NotContains(t, response.Body.String(), "input")
				require.NotContains(t, response.Body.String(), "workerId")
			}
		},
	)
	t.Run("registered routes give access tokens integration-only job scope", func(t *testing.T) {
		service := newMockjobsService(t)
		service.EXPECT().List(mock.Anything, mock.MatchedBy(func(params jobspkg.ListParams) bool {
			return params.RequesterUserID == userID && len(params.AllowedSources) == 1 &&
				params.AllowedSources[0] == jobspkg.RequesterSourceIntegration
		})).Return(jobspkg.ListResult{}, nil)
		auth := makeAuth(auth.Caller{
			UserID: userID, Credential: auth.CredentialKindAccessToken,
			AccessToken: &auth.AccessTokenCaller{TokenID: fake.UUID().V4()},
		})
		handler := server.NewTestRootHandler().RegisterJobsRoutes(NewJobsController(
			JobsControllerDeps{JobsService: service, TokenReadMiddleware: auth},
		))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/jobs?source=operator", nil))
		require.Equal(t, http.StatusOK, response.Code)
	})
	t.Run("registered routes use token-read policy", func(t *testing.T) {
		deny := middleware.AuthMiddleware(func(http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) },
			)
		})
		handler := server.NewTestRootHandler().RegisterJobsRoutes(NewJobsController(
			JobsControllerDeps{JobsService: newMockjobsService(t), TokenReadMiddleware: deny},
		))
		for _, target := range []string{"/api/v1/jobs", "/api/v1/jobs/" + fake.UUID().V4()} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
			require.Equal(t, http.StatusForbidden, response.Code)
		}
	})
	t.Run("registered detail makes inaccessible and absent jobs the same safe not-found response", func(t *testing.T) {
		service := newMockjobsService(t)
		jobIDs := []string{fake.UUID().V4(), fake.UUID().V4()}
		for _, jobID := range jobIDs {
			service.EXPECT().Get(mock.Anything, mock.MatchedBy(func(params jobspkg.GetParams) bool {
				return params.JobID == jobID && params.RequesterUserID == userID
			})).Return(nil, app.NewErrNotFound("job", jobID))
		}
		auth := makeAuth(auth.Caller{UserID: userID, Credential: auth.CredentialKindSession})
		handler := server.NewTestRootHandler().RegisterJobsRoutes(NewJobsController(
			JobsControllerDeps{JobsService: service, TokenReadMiddleware: auth},
		))
		for _, jobID := range jobIDs {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil))
			require.Equal(t, http.StatusNotFound, response.Code)
			require.Contains(t, response.Body.String(), `"code":"not_found"`)
		}
	})
	t.Run("mapping helpers discard empty filters and preserve optional fields", func(t *testing.T) {
		require.Empty(t, mapJobStatuses([]string{"", " "}))
		require.Empty(t, mapJobTypes([]string{" "}))
		require.Empty(t, mapRequesterSources([]string{""}))
		require.NotNil(t, mapJobSummary(job).Error)
		require.NotNil(t, mapJobDetail(job).Requester)
		_, err := jobReadScope(t.Context())
		require.Error(t, err)
	})
}
