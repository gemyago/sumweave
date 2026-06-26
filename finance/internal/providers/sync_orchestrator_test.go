package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
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
		lastSuccessfulWindow  domain.ProviderSyncWindow
		lastSucceededState    domain.ProviderSyncState
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
		lastSuccessfulWindow := domain.ProviderSyncWindow{
			Start: anchor.AddDate(0, 0, -30),
			End:   anchor,
		}
		targetWindow := domain.ProviderSyncWindow{
			Start: lastSuccessfulWindow.End,
			End:   lastSuccessfulWindow.End.AddDate(0, 0, 25),
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
		lastSucceededState := makeRandomProviderSyncState(fake, request.Connection)
		lastSucceededState.LastSuccessAt = &lastSuccessAt
		lastSucceededState.LastSuccessfulWindow = &lastSuccessfulWindow
		expectedWindowResults := makeWindowResults(fake)

		return orchestratorFixture{
			request:               request,
			now:                   targetWindow.End,
			lastSuccessAt:         lastSuccessAt,
			lastSuccessfulWindow:  lastSuccessfulWindow,
			lastSucceededState:    lastSucceededState,
			targetWindow:          targetWindow,
			chunks:                chunks,
			expectedWindowResults: expectedWindowResults,
		}
	}

	t.Run("orchestrate", func(t *testing.T) {
		t.Run("plans chunks and appends succeeded states in order", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			deps := makeMockDeps(t)

			var loadCalls []domain.ProviderConnectionRef
			var determineCalls []time.Time
			var determineStates []domain.ProviderSyncState
			var splitCalls []domain.ProviderSyncWindow
			var executeCalls []WindowSyncRequest
			var appendCalls []domain.ProviderSyncState

			deps.syncStateJournal.EXPECT().
				LoadLastSucceededSyncState(mock.Anything, fixture.request.Connection).
				RunAndReturn(func(
					_ context.Context,
					connection domain.ProviderConnectionRef,
				) (*domain.ProviderSyncState, error) {
					loadCalls = append(loadCalls, connection)
					return &fixture.lastSucceededState, nil
				}).
				Once()
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
				RunAndReturn(func(
					now time.Time,
					syncState *domain.ProviderSyncState,
				) (domain.ProviderSyncWindow, error) {
					determineCalls = append(determineCalls, now)
					require.NotNil(t, syncState)
					determineStates = append(determineStates, *syncState)
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
			deps.syncStateJournal.EXPECT().
				AppendSyncState(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					state domain.ProviderSyncState,
				) error {
					appendCalls = append(appendCalls, state)
					return nil
				}).
				Times(len(fixture.chunks))

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
				WithNow(func() time.Time { return fixture.now }),
			)
			require.NoError(t, err)

			result, err := orchestrator.Orchestrate(t.Context(), fixture.request)
			require.NoError(t, err)

			assert.Equal(t, []domain.ProviderConnectionRef{fixture.request.Connection}, loadCalls)
			assert.Equal(t, []time.Time{fixture.now}, determineCalls)
			require.Len(t, determineStates, 1)
			assert.Equal(t, fixture.request.Connection, determineStates[0].Connection)
			assert.NotNil(t, determineStates[0].LastAttemptAt)
			assert.Equal(t, fixture.request.JobID, determineStates[0].LastJobID)
			require.NotNil(t, determineStates[0].LastSuccessAt)
			assert.Equal(t, fixture.lastSuccessAt, *determineStates[0].LastSuccessAt)
			require.NotNil(t, determineStates[0].LastSuccessfulWindow)
			assert.Equal(t, fixture.lastSuccessfulWindow, *determineStates[0].LastSuccessfulWindow)
			assert.Equal(t, domain.ProviderSyncStats{}, determineStates[0].AggregateStats)
			assert.Equal(t, []domain.ProviderSyncWindow{fixture.targetWindow}, splitCalls)
			require.Len(t, executeCalls, len(fixture.chunks))
			require.Len(t, appendCalls, len(fixture.chunks))

			for i, executeCall := range executeCalls {
				assert.Equal(t, fixture.request.Connection, executeCall.Connection)
				assert.Equal(t, fixture.request.Secret, executeCall.Secret)
				assert.Equal(t, fixture.chunks[i], executeCall.RequestedWindow)
				require.NotNil(t, executeCall.SyncState)
				assert.Equal(t, fixture.request.Connection, executeCall.SyncState.Connection)
				assert.Equal(t, fixture.request.JobID, executeCall.JobID)
				assert.Equal(t, fixture.request.Reason, executeCall.Reason)

				appendedState := appendCalls[i]
				expectedSuccessfulWindow := domain.ProviderSyncWindow{
					Start: fixture.lastSuccessfulWindow.Start,
					End:   fixture.chunks[i].End,
				}
				require.NotNil(t, appendedState.LastSuccessfulWindow)
				assert.Equal(t, expectedSuccessfulWindow, *appendedState.LastSuccessfulWindow)
				assert.Equal(t, fixture.expectedWindowResults[i].RunID, appendedState.LastRunID)
			}

			expectedAggregateStats := mergeProviderSyncStats(
				fixture.expectedWindowResults[0].Stats,
				fixture.expectedWindowResults[1].Stats,
			)
			assert.Equal(
				t,
				expectedAggregateStats,
				appendCalls[len(appendCalls)-1].AggregateStats,
			)
			assert.Equal(t, fixture.targetWindow, result.TargetWindow)
			assert.Equal(t, fixture.chunks, result.ExecutedWindows)
			assert.Equal(t, fixture.expectedWindowResults, result.WindowResults)
			assert.Equal(t, expectedAggregateStats, result.Stats)
			assert.Equal(t, fixture.expectedWindowResults[1].Issues, result.Issues)
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
					},
					expectedErr: ErrSyncStateJournalRequired,
				},
				{
					name: "missing target window policy",
					params: SyncOrchestratorParams{
						SyncStateJournal:  deps.syncStateJournal,
						WindowChunkPolicy: deps.windowChunkPolicy,
						WindowExecutor:    deps.windowExecutor,
					},
					expectedErr: ErrTargetWindowPolicyRequired,
				},
				{
					name: "missing window chunk policy",
					params: SyncOrchestratorParams{
						SyncStateJournal:   deps.syncStateJournal,
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowExecutor:     deps.windowExecutor,
					},
					expectedErr: ErrWindowChunkPolicyRequired,
				},
				{
					name: "missing window executor",
					params: SyncOrchestratorParams{
						SyncStateJournal:   deps.syncStateJournal,
						TargetWindowPolicy: deps.targetWindowPolicy,
						WindowChunkPolicy:  deps.windowChunkPolicy,
					},
					expectedErr: ErrWindowExecutorRequired,
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
				LoadLastSucceededSyncState(mock.Anything, request.Connection).
				Once().
				Return(nil, expectedErr)

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
			)
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
				LoadLastSucceededSyncState(mock.Anything, request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
				Once().
				Return(domain.ProviderSyncWindow{}, expectedErr)

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
			)
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
				LoadLastSucceededSyncState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return(nil, expectedErr)

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
			)
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
		})

		t.Run("preserves cumulative succeeded coverage when a later chunk fails", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			expectedErr := errors.New("rate-limit-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastSucceededSyncState(mock.Anything, fixture.request.Connection).
				Once().
				Return(&domain.ProviderSyncState{
					Connection:           fixture.request.Connection,
					LastSuccessfulWindow: &fixture.lastSuccessfulWindow,
				}, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
				Once().
				Return(fixture.targetWindow, nil)
			deps.windowChunkPolicy.EXPECT().
				Split(fixture.targetWindow).
				Once().
				Return(fixture.chunks, nil)

			var appendCalls []domain.ProviderSyncState
			deps.windowExecutor.EXPECT().
				Execute(mock.Anything, mock.Anything).
				RunAndReturn(func(
					_ context.Context,
					request WindowSyncRequest,
				) (WindowSyncResult, error) {
					switch request.RequestedWindow {
					case fixture.chunks[0]:
						return WindowSyncResult{RunID: "run-success-" + fake.UUID().V4()}, nil
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
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
				WithNow(func() time.Time { return fixture.now }),
			)
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
			require.Len(t, appendCalls, 1)

			succeededCoverage := domain.ProviderSyncWindow{
				Start: fixture.lastSuccessfulWindow.Start,
				End:   fixture.chunks[0].End,
			}
			require.NotNil(t, appendCalls[0].LastSuccessfulWindow)
			assert.Equal(t, succeededCoverage, *appendCalls[0].LastSuccessfulWindow)
			assert.Empty(t, appendCalls[0].LastErrorSummary)
		})

		t.Run("does not append sync state when the first chunk execution fails", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			expectedErr := errors.New("execute-" + fake.UUID().V4())
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastSucceededSyncState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
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

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
			)
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, expectedErr)
		})

		t.Run("fails when appending a successful chunk state", func(t *testing.T) {
			fake := faker.New()
			fixture := makeTwoChunkFixture(fake)
			appendErr := errors.New("append-success-" + fake.UUID().V4())
			windowResult := makeRandomWindowSyncResult(fake)
			deps := makeMockDeps(t)
			deps.syncStateJournal.EXPECT().
				LoadLastSucceededSyncState(mock.Anything, fixture.request.Connection).
				Once().
				Return(nil, nil)
			deps.targetWindowPolicy.EXPECT().
				Determine(mock.Anything, mock.Anything).
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
			deps.syncStateJournal.EXPECT().
				AppendSyncState(mock.Anything, mock.Anything).
				Once().
				Return(appendErr)

			orchestrator, err := NewSyncOrchestrator(
				SyncOrchestratorParams{
					SyncStateJournal:   deps.syncStateJournal,
					TargetWindowPolicy: deps.targetWindowPolicy,
					WindowChunkPolicy:  deps.windowChunkPolicy,
					WindowExecutor:     deps.windowExecutor,
				},
			)
			require.NoError(t, err)

			_, err = orchestrator.Orchestrate(t.Context(), fixture.request)
			require.ErrorIs(t, err, appendErr)
			assert.Contains(t, err.Error(), "append sync state")
		})
	})
}
