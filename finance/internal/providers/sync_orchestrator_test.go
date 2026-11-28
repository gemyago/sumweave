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

		t.Run("resumes after a failed middle window without repeating durable progress", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake)
			now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
			initialWindow := domain.ProviderSyncWindow{
				Start: now.AddDate(0, 0, -70),
				End:   now,
			}
			chunkPolicy := NewOldestFirstWindowChunkPolicy()
			chunks, err := chunkPolicy.Split(initialWindow)
			require.NoError(t, err)
			require.Len(t, chunks, 3)
			firstStats := makeRandomProviderSyncStats(fake)
			failedWindowErr := errors.New("middle-window-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			var latestState *domain.ProviderSyncState
			var executedWindows []domain.ProviderSyncWindow
			failedOnce := true

			deps.syncStateJournal.EXPECT().
				LoadLastState(mock.Anything, request.Connection).
				RunAndReturn(func(context.Context, domain.ProviderConnectionRef) (*domain.ProviderSyncState, error) {
					return latestState, nil
				}).
				Once()
			deps.syncStateJournal.EXPECT().
				AppendSyncState(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, state domain.ProviderSyncState) error {
					latestState = &state
					return nil
				}).
				Once()
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, windowRequest WindowSyncRequest) (WindowSyncResult, error) {
					executedWindows = append(executedWindows, windowRequest.RequestedWindow)
					switch windowRequest.RequestedWindow {
					case chunks[0]:
						return WindowSyncResult{RunID: "run-first-" + fake.UUID().V4(), Stats: firstStats}, nil
					case chunks[1]:
						if failedOnce {
							failedOnce = false
							return WindowSyncResult{}, failedWindowErr
						}
						return WindowSyncResult{RunID: "run-retry-middle-" + fake.UUID().V4()}, nil
					case chunks[2]:
						return WindowSyncResult{RunID: "run-retry-last-" + fake.UUID().V4()}, nil
					default:
						t.Fatalf("unexpected requested window: %#v", windowRequest.RequestedWindow)
						return WindowSyncResult{}, nil
					}
				})

			orchestrator, err := NewSyncOrchestrator(SyncOrchestratorParams{
				SyncStateJournal:   deps.syncStateJournal,
				TargetWindowPolicy: NewCheckpointTargetWindowPolicy(),
				WindowChunkPolicy:  chunkPolicy,
				WindowExecutor:     deps.windowExecutor,
				Logger:             slog.New(slog.DiscardHandler),
			}, WithNow(func() time.Time { return now }))
			require.NoError(t, err)

			initialRequest := request
			initialRequest.WindowStart = &initialWindow.Start
			initialRequest.WindowEnd = &initialWindow.End
			_, err = orchestrator.Orchestrate(t.Context(), initialRequest)
			require.ErrorIs(t, err, failedWindowErr)
			require.NotNil(t, latestState)
			assert.Equal(t, chunks[1], latestState.Window)
			assert.Nil(t, latestState.SucceededAt)
			assert.Equal(t, firstStats, latestState.AggregateStats)

			retryResult, err := orchestrator.Orchestrate(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncWindow{Start: chunks[1].Start, End: now}, retryResult.TargetWindow)
			assert.Equal(t, chunks[1:], retryResult.ExecutedWindows)
			assert.Equal(t, []domain.ProviderSyncWindow{
				chunks[0], chunks[1], chunks[1], chunks[2],
			}, executedWindows)
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
	t.Run("interrupted replay resumes from committed progress and failed progress stops execution", func(t *testing.T) {
		fake := faker.New()
		anchor := time.Now().Truncate(time.Second)
		floor := anchor.AddDate(-3, 0, 0)
		request := makeRequest(fake)
		request.ReplayFrom = &floor
		journal := NewMockSyncStateJournal(t)
		recent := makeRandomProviderSyncState(fake, request.Connection)
		recent.Window = domain.ProviderSyncWindow{Start: anchor.AddDate(0, 0, -10), End: anchor}
		recent.SucceededAt = &anchor
		journal.EXPECT().LoadLastState(mock.Anything, request.Connection).Return(&recent, nil)
		journal.EXPECT().AppendSyncState(mock.Anything, mock.Anything).Return(nil).Once()
		executor := NewMockWindowExecutor(t)
		var fetched, committed []domain.ProviderSyncWindow
		interrupted := false
		executor.EXPECT().
			Execute(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, r WindowSyncRequest) (WindowSyncResult, error) {
				fetched = append(fetched, r.RequestedWindow)
				if len(committed) == 2 && !interrupted {
					interrupted = true
					return WindowSyncResult{}, context.DeadlineExceeded
				}
				return WindowSyncResult{}, nil
			})
		request.OnWindowCommitted = func(_ context.Context, window domain.ProviderSyncWindow) error {
			committed = append(committed, window)
			floor = window.End
			return nil
		}
		orchestrator, err := NewSyncOrchestrator(SyncOrchestratorParams{
			SyncStateJournal: journal, TargetWindowPolicy: NewCheckpointTargetWindowPolicy(),
			WindowChunkPolicy: NewOldestFirstWindowChunkPolicy(), WindowExecutor: executor,
			Logger: slog.New(slog.DiscardHandler),
		}, WithNow(func() time.Time { return anchor }))
		require.NoError(t, err)
		_, err = orchestrator.Orchestrate(t.Context(), request)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Len(t, committed, 2)
		require.Equal(t, committed[1].End, floor)
		failedWindow := fetched[2]
		result, err := orchestrator.Orchestrate(t.Context(), request)
		require.NoError(t, err)
		require.Equal(t, failedWindow, fetched[3])
		require.Equal(t, committed[1].End, result.TargetWindow.Start)
		require.Equal(t, anchor, result.ExecutedWindows[len(result.ExecutedWindows)-1].End)
		for i := 1; i < len(committed); i++ {
			require.Equal(t, committed[i-1].End, committed[i].Start)
		}
		progressErr := errors.New(fake.UUID().V4())
		request.OnWindowCommitted = func(context.Context, domain.ProviderSyncWindow) error { return progressErr }
		request.ReplayFrom = nil
		before := len(fetched)
		_, err = orchestrator.Orchestrate(t.Context(), request)
		require.ErrorIs(t, err, progressErr)
		require.Len(t, fetched, before+1, "failed progress persistence must stop subsequent chunks")
	})
	t.Run("real planning and execution honor replay floors only for unbounded runs", func(t *testing.T) {
		fake := faker.New()
		anchor := time.Now().AddDate(0, -fake.IntBetween(1, 6), 0).Truncate(time.Second)
		replayFrom := anchor.AddDate(-3, 0, 0)
		boundedStart, boundedEnd := anchor.AddDate(0, 0, -7), anchor.AddDate(0, 0, -1)
		window := func(start, end time.Time) domain.ProviderSyncWindow {
			return domain.ProviderSyncWindow{Start: start, End: end}
		}
		for _, testCase := range []struct {
			name        string
			start, end  *time.Time
			expected    domain.ProviderSyncWindow
			singleChunk bool
		}{
			{name: "unbounded", expected: window(replayFrom, anchor)},
			{name: "explicit range", start: &boundedStart, end: &boundedEnd, expected: window(boundedStart, boundedEnd), singleChunk: true},
			{name: "explicit start", start: &boundedStart, expected: window(boundedStart, anchor), singleChunk: true},
			{name: "explicit end", end: &boundedEnd, expected: window(boundedEnd.AddDate(0, 0, -30), boundedEnd), singleChunk: true},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				request := makeRequest(fake)
				request.ReplayFrom, request.WindowStart, request.WindowEnd = &replayFrom, testCase.start, testCase.end
				lastState := makeRandomProviderSyncState(fake, request.Connection)
				succeededAt := anchor.AddDate(0, 0, -2)
				lastState.SucceededAt = &succeededAt
				lastState.Window = window(anchor.AddDate(0, 0, -15), succeededAt)
				journal := NewMockSyncStateJournal(t)
				if testCase.start == nil {
					journal.EXPECT().LoadLastState(mock.Anything, request.Connection).Return(&lastState, nil).Once()
				}
				connector := NewMockConnector(t)
				connector.EXPECT().ConnectorID().Return(request.Connection.ConnectorID).Once()
				var fetched, applied, committed []domain.ProviderSyncWindow
				connector.EXPECT().
					Fetch(mock.Anything, mock.Anything).
					RunAndReturn(func(_ context.Context, fetch FetchRequest) (domain.ProviderSyncBatch, error) {
						require.Equal(t, request.Connection, fetch.Connection)
						require.Equal(t, request.Secret, fetch.Secret)
						fetched = append(fetched, fetch.RequestedWindow)
						return domain.ProviderSyncBatch{
							Connection:      fetch.Connection,
							RequestedWindow: fetch.RequestedWindow,
						}, nil
					})
				request.OnWindowCommitted = func(_ context.Context, window domain.ProviderSyncWindow) error {
					require.Equal(t, window, applied[len(applied)-1], "progress follows committed apply")
					committed = append(committed, window)
					return nil
				}
				store := NewMockWindowSyncStore(t)
				store.EXPECT().
					LoadExistingWindow(mock.Anything, request.Connection, mock.Anything, mock.Anything).
					RunAndReturn(func(_ context.Context, connection domain.ProviderConnectionRef, rangeWindow domain.ProviderSyncWindow, _ []ProviderTransactionIdentity) (ExistingWindowSnapshot, error) {
						return ExistingWindowSnapshot{Connection: connection, SnapshotWindow: rangeWindow}, nil
					})
				store.EXPECT().
					ApplySync(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					RunAndReturn(func(_ context.Context, _ ProviderDiffPlan, _ ApplyPlan, states ...domain.ProviderSyncState) (domain.ProviderSyncStats, error) {
						require.Len(t, states, 1)
						require.NotNil(t, states[0].SucceededAt)
						applied = append(applied, states[0].Window)
						return domain.ProviderSyncStats{}, nil
					})
				executor, err := NewWindowSyncExecutor(
					WithConnectors(connector),
					WithWindowSyncStore(store),
					WithWindowSyncExecutorNow(func() time.Time { return anchor }),
				)
				require.NoError(t, err)
				orchestrator, err := NewSyncOrchestrator(
					SyncOrchestratorParams{
						SyncStateJournal:   journal,
						TargetWindowPolicy: NewCheckpointTargetWindowPolicy(),
						WindowChunkPolicy:  NewOldestFirstWindowChunkPolicy(),
						WindowExecutor:     executor,
						Logger:             slog.New(slog.DiscardHandler),
					},
					WithNow(func() time.Time { return anchor }),
				)
				require.NoError(t, err)
				result, err := orchestrator.Orchestrate(t.Context(), request)
				require.NoError(t, err)
				require.NotEmpty(t, fetched)
				require.Equal(t, testCase.expected, result.TargetWindow)
				require.Equal(t, testCase.expected.Start, fetched[0].Start)
				require.Equal(t, testCase.expected.End, fetched[len(fetched)-1].End)
				require.Equal(t, fetched, applied)
				require.Equal(t, applied, committed)
				require.Equal(t, fetched, result.ExecutedWindows)
				for i, executed := range fetched {
					require.False(t, executed.End.After(executed.Start.AddDate(0, 0, 30)))
					if i > 0 {
						require.Equal(t, fetched[i-1].End, executed.Start)
					}
				}
				if testCase.singleChunk {
					require.Len(t, fetched, 1)
				} else {
					require.Greater(t, len(fetched), 30)
				}
			})
		}
	})
}
