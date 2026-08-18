package providers

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncOrchestrator(t *testing.T) {
	type mockDeps struct {
		syncStateJournal   *MockSyncStateJournal
		targetWindowPolicy *MockTargetWindowPolicy
		windowChunkPolicy  *MockWindowChunkPolicy
		windowExecutor     *MockWindowExecutor
	}

	type orchestratorFixture struct {
		request               SyncOrchestrationRequest
		now                   time.Time
		lastSuccessAt         time.Time
		lastWindow            domain.ProviderSyncWindow
		lastState             domain.ProviderSyncState
		targetWindow          domain.ProviderSyncWindow
		chunks                []domain.ProviderSyncWindow
		expectedWindowResults []WindowSyncResult
	}

	makeMockDeps := func(t *testing.T) mockDeps {
		return mockDeps{
			syncStateJournal:   NewMockSyncStateJournal(t),
			targetWindowPolicy: NewMockTargetWindowPolicy(t),
			windowChunkPolicy:  NewMockWindowChunkPolicy(t),
			windowExecutor:     NewMockWindowExecutor(t),
		}
	}

	makeOrchestratorParams := func(deps mockDeps) SyncOrchestratorParams {
		return SyncOrchestratorParams{
			SyncStateJournal:   deps.syncStateJournal,
			TargetWindowPolicy: deps.targetWindowPolicy,
			WindowChunkPolicy:  deps.windowChunkPolicy,
			WindowExecutor:     deps.windowExecutor,
			Logger:             slog.New(slog.DiscardHandler),
		}
	}

	makeRequest := func(fake faker.Faker) SyncOrchestrationRequest {
		providerID := domain.ProviderIDMonobank

		return SyncOrchestrationRequest{
			Connection: makeRandomProviderConnectionRef(
				fake,
				providerID,
				domain.ProviderConnectorIDMonobank,
			),
			Secret: makeRandomConnectionSecret(fake, providerID),
			JobID:  "job-" + fake.UUID().V4(),
			Reason: "scheduled-" + fake.Lorem().Word(),
		}
	}

	makeAnchorTime := func(fake faker.Faker) time.Time {
		return time.Date(
			2026,
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 20),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			0,
			0,
			time.UTC,
		)
	}

	makeWindowResults := func(fake faker.Faker) []WindowSyncResult {
		firstResult := makeRandomWindowSyncResult(fake)
		firstResult.RunID = "run-case1-" + fake.UUID().V4()
		firstResult.Issues = nil

		secondResult := makeRandomWindowSyncResult(fake)
		secondResult.RunID = "run-case2-" + fake.UUID().V4()

		return []WindowSyncResult{firstResult, secondResult}
	}

	makeTwoChunkFixture := func(fake faker.Faker) orchestratorFixture {
		request := makeRequest(fake)
		anchor := makeAnchorTime(fake)
		lastSuccessAt := anchor.Add(-6 * time.Hour)
		lastWindow := domain.ProviderSyncWindow{
			Start: anchor.AddDate(0, 0, -30),
			End:   anchor,
		}
		targetWindow := domain.ProviderSyncWindow{
			Start: lastWindow.End,
			End:   lastWindow.End.AddDate(0, 0, 25),
		}
		chunks := []domain.ProviderSyncWindow{
			{
				Start: targetWindow.Start,
				End:   targetWindow.Start.AddDate(0, 0, 15),
			},
			{
				Start: targetWindow.Start.AddDate(0, 0, 15),
				End:   targetWindow.End,
			},
		}
		lastState := makeRandomProviderSyncState(fake, request.Connection)
		lastState.SucceededAt = &lastSuccessAt
		lastState.Window = lastWindow
		expectedWindowResults := makeWindowResults(fake)

		return orchestratorFixture{
			request:               request,
			now:                   targetWindow.End,
			lastSuccessAt:         lastSuccessAt,
			lastWindow:            lastWindow,
			lastState:             lastState,
			targetWindow:          targetWindow,
			chunks:                chunks,
			expectedWindowResults: expectedWindowResults,
		}
	}

	t.Run("orchestrate", func(t *testing.T) {
		t.Run("plans chunks and appends attempt states in order", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			deps := makeMockDeps(t)

			var loadCalls []domain.ProviderConnectionRef
			var determineCalls []TargetWindowRequest
			var determineStates []domain.ProviderSyncState
			var splitCalls []domain.ProviderSyncWindow
			var executeCalls []WindowSyncRequest

			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				RunAndReturn(func(
					_ context.Context,
					connection domain.ProviderConnectionRef,
				) (*domain.ProviderSyncState, error) {
					loadCalls = append(loadCalls, connection)
					return &fixture.lastState, nil
				}).
				Once()
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				RunAndReturn(func(
					request TargetWindowRequest,
				) (domain.ProviderSyncWindow, error) {
					determineCalls = append(determineCalls, request)
					require.NotNil(t, request.State)
					determineStates = append(determineStates, *request.State)
					return fixture.targetWindow, nil
				}).
				Once()
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				RunAndReturn(func(
					window domain.ProviderSyncWindow,
				) ([]domain.ProviderSyncWindow, error) {
					splitCalls = append(splitCalls, window)
					return append([]domain.ProviderSyncWindow(nil), fixture.chunks...), nil
				}).
				Once()
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					request WindowSyncRequest,
				) (WindowSyncResult, error) {
					executeCalls = append(executeCalls, request)
					return fixture.expectedWindowResults[len(executeCalls)-1], nil
				}).
				Times(len(fixture.chunks))

			orchestrator, err := NewSyncOrchestrator(
				makeOrchestratorParams(deps),
				WithNow(func() time.Time { return fixture.now }),
			)
			require.NoError(t, err)

			result, err := orchestrator.Orchestrate(t.Context(), fixture.request)
			require.NoError(t, err)

			assert.Equal(t, []domain.ProviderConnectionRef{fixture.request.Connection}, loadCalls)
			assert.Equal(t, []TargetWindowRequest{{
				Now:   fixture.now,
				State: &fixture.lastState,
			}}, determineCalls)
			require.Len(t, determineStates, 1)
			assert.Equal(t, fixture.lastState, determineStates[0])
			assert.Equal(t, []domain.ProviderSyncWindow{fixture.targetWindow}, splitCalls)
			require.Len(t, executeCalls, len(fixture.chunks))

			for i, executeCall := range executeCalls {
				assert.Equal(t, fixture.request.Connection, executeCall.Connection)
				assert.Equal(t, fixture.request.Secret, executeCall.Secret)
				assert.Equal(t, fixture.chunks[i], executeCall.RequestedWindow)
				require.NotNil(t, executeCall.SyncState)
				assert.Equal(t, fixture.request.Connection, executeCall.SyncState.Connection)
				assert.Equal(t, fixture.request.JobID, executeCall.JobID)
				assert.Equal(t, fixture.request.Reason, executeCall.Reason)
				assert.Equal(t, fixture.chunks[i], executeCall.SyncState.Window)
				assert.NotNil(t, executeCall.SyncState.AttemptedAt)
			}

			expectedAggregateStats := mergeProviderSyncStats(
				fixture.expectedWindowResults[0].Stats,
				fixture.expectedWindowResults[1].Stats,
			)
			assert.Equal(t, fixture.targetWindow, result.TargetWindow)
			assert.Equal(t, fixture.chunks, result.ExecutedWindows)
			assert.Equal(t, fixture.expectedWindowResults, result.WindowResults)
			assert.Equal(t, expectedAggregateStats, result.Stats)
			assert.Equal(t, fixture.expectedWindowResults[1].Issues, result.Issues)
		})

		t.Run("passes optional bounds to target planning", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			start := fixture.now.AddDate(0, 0, -fake.IntBetween(1, 29))
			end := fixture.now.AddDate(0, 0, fake.IntBetween(1, 29))

			testCases := []struct {
				name          string
				windowStart   *time.Time
				windowEnd     *time.Time
				loadLastState bool
			}{
				{
					name:          "start only",
					windowStart:   &start,
					loadLastState: false,
				},
				{
					name:          "end only",
					windowEnd:     &end,
					loadLastState: true,
				},
				{
					name:          "complete explicit window",
					windowStart:   &start,
					windowEnd:     &end,
					loadLastState: false,
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					deps := makeMockDeps(t)
					request := fixture.request
					request.WindowStart = testCase.windowStart
					request.WindowEnd = testCase.windowEnd

					var lastState *domain.ProviderSyncState
					if testCase.loadLastState {
						lastState = &fixture.lastState
						deps.syncStateJournal.EXPECT().
							LoadLastState(mock.Anything, request.Connection).
							Once().
							Return(lastState, nil)
					}
					deps.targetWindowPolicy.EXPECT().
						Determine(TargetWindowRequest{
							Now:         fixture.now,
							State:       lastState,
							WindowStart: testCase.windowStart,
							WindowEnd:   testCase.windowEnd,
						}).
						Once().
						Return(fixture.targetWindow, nil)
					deps.windowChunkPolicy.EXPECT().
						Split(fixture.targetWindow).
						Once().
						Return(nil, nil)

					orchestrator, err := NewSyncOrchestrator(
						makeOrchestratorParams(deps),
						WithNow(func() time.Time { return fixture.now }),
					)
					require.NoError(t, err)

					result, err := orchestrator.Orchestrate(t.Context(), request)
					require.NoError(t, err)
					assert.Equal(t, fixture.targetWindow, result.TargetWindow)
				})
			}
		})

		t.Run("constructor fails when a required dependency is missing", func(t *testing.T) {
			deps := makeMockDeps(t)

			testCases := []struct {
				name        string
				params      SyncOrchestratorParams
				expectedErr error
			}{
				{
					name: "missing sync state journal",
					params: SyncOrchestratorParams{
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowChunkPolicy:  deps.windowChunkPolicy,
						WindowExecutor:     deps.windowExecutor,
						Logger:             slog.New(slog.DiscardHandler),
					},
					expectedErr: ErrSyncStateJournalRequired,
				},
				{
					name: "missing target window policy",
					params: SyncOrchestratorParams{
						SyncStateJournal:  deps.syncStateJournal,
						WindowChunkPolicy: deps.windowChunkPolicy,
						WindowExecutor:    deps.windowExecutor,
						Logger:            slog.New(slog.DiscardHandler),
					},
					expectedErr: ErrTargetWindowPolicyRequired,
				},
				{
					name: "missing window chunk policy",
					params: SyncOrchestratorParams{
						SyncStateJournal:   deps.syncStateJournal,
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowExecutor:     deps.windowExecutor,
						Logger:             slog.New(slog.DiscardHandler),
					},
					expectedErr: ErrWindowChunkPolicyRequired,
				},
				{
					name: "missing window executor",
					params: SyncOrchestratorParams{
						SyncStateJournal:   deps.syncStateJournal,
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowChunkPolicy:  deps.windowChunkPolicy,
						Logger:             slog.New(slog.DiscardHandler),
					},
					expectedErr: ErrWindowExecutorRequired,
				},
				{
					name: "missing logger",
					params: SyncOrchestratorParams{
						SyncStateJournal:   deps.syncStateJournal,
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowChunkPolicy:  deps.windowChunkPolicy,
						WindowExecutor:     deps.windowExecutor,
					},
					expectedErr: ErrLoggerRequired,
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					orchestrator, err := NewSyncOrchestrator(testCase.params)
					assert.Nil(t, orchestrator)
					require.ErrorIs(t, err, testCase.expectedErr)
				})
			}
		})

		t.Run("wraps journal load errors with orchestration context", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake)
			expectedErr := errors.New("boom-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, request.Connection).
				Once().
				Return(nil, expectedErr)

			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			assert.Contains(t, err.Error(), "load sync state")
		})

		t.Run("does not append sync state when target window planning fails", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake)
			expectedErr := errors.New("determine-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				Once().
				Return(domain.ProviderSyncWindow{}, expectedErr)

			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
		})

		t.Run("does not append sync state when chunk splitting fails", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			expectedErr := errors.New("split-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return(nil, expectedErr)

			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
		})

		t.Run("appends a failed latest state when a later chunk fails", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			expectedErr := errors.New("rate-limit-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				Once().
				Return(&domain.ProviderSyncState{
					Connection: fixture.request.Connection,
					Window:     fixture.lastWindow,
				}, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return(fixture.chunks, nil)

			var appendCalls []domain.ProviderSyncState
			firstStats := makeRandomProviderSyncStats(fake)
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					request WindowSyncRequest,
				) (WindowSyncResult, error) {
					switch request.RequestedWindow {
					case fixture.chunks[0]:
						return WindowSyncResult{
							RunID: "run-success-" + fake.UUID().V4(),
							Stats: firstStats,
						}, nil
					default:
						return WindowSyncResult{}, expectedErr
					}
				}).
				Times(len(fixture.chunks))
			deps.syncStateJournal.EXPECT().
				AppendSyncState(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					state domain.ProviderSyncState,
				) error {
					appendCalls = append(appendCalls, state)
					return nil
				}).
				Once()

			orchestrator, err := NewSyncOrchestrator(
				makeOrchestratorParams(deps),
				WithNow(func() time.Time { return fixture.now }),
			)
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
			require.Len(t, appendCalls, 1)
			assert.Equal(t, fixture.chunks[1], appendCalls[0].Window)
			assert.Nil(t, appendCalls[0].SucceededAt)
			assert.Equal(t, expectedErr.Error(), appendCalls[0].ErrorSummary)
			assert.Equal(t, firstStats, appendCalls[0].AggregateStats)
		})

		t.Run("appends a failed latest state when the first chunk execution fails", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			expectedErr := errors.New("execute-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return([]domain.ProviderSyncWindow{fixture.chunks[0]}, nil)
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				Once().
				Return(WindowSyncResult{}, expectedErr)

			var appendCalls []domain.ProviderSyncState
			deps.syncStateJournal.EXPECT().
				AppendSyncState(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					state domain.ProviderSyncState,
				) error {
					appendCalls = append(appendCalls, state)
					return nil
				}).
				Once()

			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
			require.Len(t, appendCalls, 1)
			assert.Equal(t, fixture.chunks[0], appendCalls[0].Window)
			assert.Nil(t, appendCalls[0].SucceededAt)
			assert.Equal(t, expectedErr.Error(), appendCalls[0].ErrorSummary)
		})

		t.Run("does not append successful chunk states through the standalone journal", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			windowResult := makeRandomWindowSyncResult(fake)
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return([]domain.ProviderSyncWindow{fixture.targetWindow}, nil)
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				Once().
				Return(windowResult, nil)
			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			result, err := orchestrator.Orchestrate(t.Context(), fixture.request)
			require.NoError(t, err)
			assert.Equal(t, windowResult.Stats, result.Stats)
		})

		t.Run("passes the latest failed state to target planning", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			failedState := domain.ProviderSyncState{
				Connection: fixture.request.Connection,
				Window:     fixture.chunks[0],
			}
			deps := makeMockDeps(t)

			var determineState *domain.ProviderSyncState
			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, fixture.request.Connection).
				Once().
				Return(&failedState, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything).
				RunAndReturn(func(request TargetWindowRequest) (domain.ProviderSyncWindow, error) {
					determineState = request.State
					return fixture.targetWindow, nil
				}).
				Once()
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return(nil, errors.New("stop-after-plan-"+fake.UUID().V4()))

			orchestrator, err := NewSyncOrchestrator(makeOrchestratorParams(deps))
			require.NoError(t, err)

			_, _ = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.NotNil(t, determineState)
			assert.Equal(t, failedState, *determineState)
		})
	})
}
