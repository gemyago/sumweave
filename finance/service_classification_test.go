package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMatchClassificationRule(t *testing.T) {
	t.Run("uses the first normalized literal match and preserves internal text", func(t *testing.T) {
		rules := []domain.ClassificationRule{
			{ID: "contains", MatchType: domain.ClassificationMatchTypeContains, Condition: "NETFLIX"},
			{ID: "exact", MatchType: domain.ClassificationMatchTypeExact, Condition: "NETFLIX.COM"},
		}
		match := matchClassificationRule("  netflix.com payment  ", rules)
		if assert.NotNil(t, match) {
			assert.Equal(t, "contains", match.ID)
		}
		assert.Nil(t, matchClassificationRule("NETFLIX COM", []domain.ClassificationRule{{
			MatchType: domain.ClassificationMatchTypeExact, Condition: "NETFLIX.COM",
		}}))
		assert.Nil(t, matchClassificationRule("   ", rules))
	})

	t.Run("matches Unicode contains conditions and skips blank rule conditions", func(t *testing.T) {
		fake := faker.New()
		condition := "ŻÓŁĆ-" + fake.Lorem().Word()
		rules := []domain.ClassificationRule{
			{
				ID:        "blank-" + fake.UUID().V4(),
				MatchType: domain.ClassificationMatchTypeContains,
				Condition: "  ",
			},
			{
				ID:        "contains-" + fake.UUID().V4(),
				MatchType: domain.ClassificationMatchTypeContains,
				Condition: condition,
			},
		}
		match := matchClassificationRule(" payment żółć-"+condition[len("ŻÓŁĆ-"):]+"! ", rules)
		if assert.NotNil(t, match) {
			assert.Equal(t, rules[1].ID, match.ID)
		}
		assert.Nil(t, matchClassificationRule("plain", []domain.ClassificationRule{{
			MatchType: domain.ClassificationMatchTypeContains, Condition: "",
		}}))
	})
}

func TestClassificationService(t *testing.T) {
	t.Run("submits an offset-bearing range as future observed work", func(t *testing.T) {
		fake := faker.New()
		access := newMockaccessGuardStore(t)
		rules := newMockclassificationRuleStore(t)
		transactions := newMockclassificationTransactionStore(t)
		categories := newMockclassificationCategoryStore(t)
		publisher := NewMockSemanticCommandPublisher(t)
		start := time.Date(2026, time.September, 6, 9, 30, 0, 0, time.FixedZone("east", 3*60*60))
		end := start.Add(2 * time.Hour)
		params := SubmitClassificationParams{
			ActorUserID:       "user-" + fake.UUID().V4(),
			TenantID:          "tenant-" + fake.UUID().V4(),
			RangeStart:        start,
			RangeEndExclusive: end,
		}
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		expected := DispatchReference{MessageID: fake.UUID().V4()}
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.MatchedBy(func(command SemanticCommand) bool {
			if command.Topic != ClassificationExplicitCommandTopic {
				return false
			}
			var payload ClassificationExplicitCommand
			return json.Unmarshal(command.Payload, &payload) == nil &&
				payload.TenantID == params.TenantID &&
				payload.RangeStart.Equal(start) &&
				payload.RangeStart.Format(time.RFC3339Nano) == start.Format(time.RFC3339Nano) &&
				payload.RangeEndExclusive.Equal(end) &&
				payload.Requester == (CommandRequester{
					UserID: params.ActorUserID,
					Source: CommandRequesterSourceOperator,
				})
		})).Return(expected, nil).Once()
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
			Categories:   categories,
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
			Now:          time.Now,
		}, WithClassificationServiceCommandPublisher(publisher))
		require.NoError(t, err)
		result, err := service.Submit(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, ClassificationJobRef{ID: expected.MessageID}, result)
	})

	t.Run("rejects invalid explicit ranges before publication", func(t *testing.T) {
		fake := faker.New()
		access := newMockaccessGuardStore(t)
		now := time.Now()
		params := SubmitClassificationParams{
			ActorUserID:       fake.UUID().V4(),
			TenantID:          fake.UUID().V4(),
			RangeStart:        now,
			RangeEndExclusive: now,
		}
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        newMockclassificationRuleStore(t),
			Transactions: newMockclassificationTransactionStore(t),
			Categories:   newMockclassificationCategoryStore(t),
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
			Now:          time.Now,
		}, WithClassificationServiceCommandPublisher(NewMockSemanticCommandPublisher(t)))
		require.NoError(t, err)
		_, err = service.Submit(t.Context(), params)
		require.ErrorIs(t, err, ErrInvalidTimestampRange)
	})

	t.Run("creates a new publication identity for each explicit submission", func(t *testing.T) {
		fake := faker.New()
		access := newMockaccessGuardStore(t)
		publisher := NewMockSemanticCommandPublisher(t)
		start := time.Date(2026, time.September, 6, 9, 30, 0, 0, time.FixedZone("east", 3*60*60))
		params := SubmitClassificationParams{
			ActorUserID:       "user-" + fake.UUID().V4(),
			TenantID:          "tenant-" + fake.UUID().V4(),
			RangeStart:        start,
			RangeEndExclusive: start.Add(time.Hour),
		}
		access.EXPECT().IsTenantMember(mock.Anything, params.TenantID, params.ActorUserID).Return(true, nil).Twice()
		commands := make([]SemanticCommand, 0, 2)
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.MatchedBy(func(command SemanticCommand) bool {
			return command.Topic == ClassificationExplicitCommandTopic
		})).RunAndReturn(func(_ context.Context, command SemanticCommand) (DispatchReference, error) {
			commands = append(commands, command)
			return DispatchReference{MessageID: "message-" + fake.UUID().V4()}, nil
		}).Twice()
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        newMockclassificationRuleStore(t),
			Transactions: newMockclassificationTransactionStore(t),
			Categories:   newMockclassificationCategoryStore(t),
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
			Now:          time.Now,
		}, WithClassificationServiceCommandPublisher(publisher))
		require.NoError(t, err)
		first, err := service.Submit(t.Context(), params)
		require.NoError(t, err)
		second, err := service.Submit(t.Context(), params)
		require.NoError(t, err)
		assert.NotEqual(t, first, second)
		require.Len(t, commands, 2)
		assert.NotEmpty(t, commands[0].IdempotencyKey)
		assert.NotEqual(t, commands[0].IdempotencyKey, commands[1].IdempotencyKey)
		assert.Equal(t, commands[0].Payload, commands[1].Payload)
	})

	t.Run("loads rules once, advances keyset batches, and counts committed assignments", func(t *testing.T) {
		rules := newMockclassificationRuleStore(t)
		transactions := newMockclassificationTransactionStore(t)
		categories := newMockclassificationCategoryStore(t)
		access := newMockaccessGuardStore(t)
		now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := ClassificationParams{
			TenantID:          "tenant-a",
			RangeStart:        now.Add(-time.Hour),
			RangeEndExclusive: now,
			MessageID:         "message-a",
		}
		rules.EXPECT().ListClassificationRules(t.Context(), params.TenantID, "").Return([]domain.ClassificationRule{{
			ID: "rule-a", TenantID: params.TenantID, MatchType: domain.ClassificationMatchTypeContains,
			Condition: "match", CategoryID: "category-a",
		}}, nil).Once()
		first := domain.Transaction{ID: "transaction-a", TenantID: params.TenantID, Description: "matching description"}
		second := domain.Transaction{ID: "transaction-b", TenantID: params.TenantID, Description: "other description"}
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID: params.TenantID, RangeStart: params.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
			}).
			Return([]domain.Transaction{first, second}, nil).
			Once()
		categories.EXPECT().
			GetCategory(t.Context(), "category-a").
			Return(&domain.Category{ID: "category-a", TenantID: params.TenantID}, nil).
			Once()
		transactions.EXPECT().AssignClassification(t.Context(), persistence.AssignClassificationParams{
			TenantID: params.TenantID, TransactionID: first.ID, CategoryID: "category-a", UpdatedAt: now,
		}).Return(true, nil).Once()
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          params.TenantID,
				RangeStart:        params.RangeStart,
				RangeEndExclusive: params.RangeEndExclusive,
				AfterID:           second.ID,
			}).
			Return(nil, nil).
			Once()
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
			Categories:   categories,
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
			Now:          func() time.Time { return now },
		})
		if assert.NoError(t, err) {
			counts, classifyErr := service.Classify(t.Context(), params)
			assert.NoError(t, classifyErr)
			assert.Equal(t, ClassificationAttemptCounts{Classified: 1, Unmatched: 1}, counts)
		}
	})

	t.Run("uses only the winning rule tags for one committed assignment", func(t *testing.T) {
		fake := faker.New()
		rules := newMockclassificationRuleStore(t)
		transactions := newMockclassificationTransactionStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		access := newMockaccessGuardStore(t)
		now := time.Date(2026, time.September, 9, 15, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := ClassificationParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now.Add(-time.Hour), RangeEndExclusive: now,
		}
		firstTagID := "tag-first-" + fake.UUID().V4()
		secondTagID := "tag-second-" + fake.UUID().V4()
		firstCategoryID := "category-first-" + fake.UUID().V4()
		secondCategoryID := "category-second-" + fake.UUID().V4()
		transaction := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "matching description"}
		rules.EXPECT().ListClassificationRules(t.Context(), params.TenantID, "").Return([]domain.ClassificationRule{
			{
				ID:         "rule-first-" + fake.UUID().V4(),
				MatchType:  domain.ClassificationMatchTypeContains,
				Condition:  "matching",
				CategoryID: firstCategoryID,
				TagIDs:     []string{firstTagID},
			},
			{
				ID:         "rule-second-" + fake.UUID().V4(),
				MatchType:  domain.ClassificationMatchTypeContains,
				Condition:  "matching",
				CategoryID: secondCategoryID,
				TagIDs:     []string{secondTagID},
			},
		}, nil).Once()
		transactions.EXPECT().ListEligibleClassificationTransactions(
			t.Context(),
			persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          params.TenantID,
				RangeStart:        params.RangeStart,
				RangeEndExclusive: params.RangeEndExclusive,
			},
		).Return([]domain.Transaction{transaction}, nil).Once()
		categories.EXPECT().GetCategory(t.Context(), firstCategoryID).Return(
			&domain.Category{ID: firstCategoryID, TenantID: params.TenantID},
			nil,
		).Once()
		tags.EXPECT().GetTag(t.Context(), firstTagID).Return(
			&domain.Tag{ID: firstTagID, TenantID: params.TenantID},
			nil,
		).Once()
		transactions.EXPECT().AssignClassification(t.Context(), persistence.AssignClassificationParams{
			TenantID:      params.TenantID,
			TransactionID: transaction.ID,
			CategoryID:    firstCategoryID,
			TagIDs:        []string{firstTagID},
			UpdatedAt:     now,
		}).Return(true, nil).Once()
		transactions.EXPECT().ListEligibleClassificationTransactions(
			t.Context(),
			persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          params.TenantID,
				RangeStart:        params.RangeStart,
				RangeEndExclusive: params.RangeEndExclusive,
				AfterID:           transaction.ID,
			},
		).Return(nil, nil).Once()
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access: access, Rules: rules, Transactions: transactions, Categories: categories, Tags: tags,
			Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
		})
		require.NoError(t, err)
		counts, err := service.Classify(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, counts)
	})

	t.Run("makes unavailable rule tags terminal and tag lookup failures retryable", func(t *testing.T) {
		fake := faker.New()
		makeService := func(t *testing.T) (*ClassificationService, *mockclassificationRuleStore, *mockclassificationTransactionStore, *mockclassificationCategoryStore, *mockclassificationTagStore, ClassificationParams, domain.Transaction, string, string) {
			t.Helper()
			now := time.Date(2026, time.September, 9, 16, 0, 0, 0, time.FixedZone("test", 2*60*60))
			params := ClassificationParams{
				TenantID:          "tenant-" + fake.UUID().V4(),
				RangeStart:        now.Add(-time.Hour),
				RangeEndExclusive: now,
			}
			rules := newMockclassificationRuleStore(t)
			transactions := newMockclassificationTransactionStore(t)
			categories := newMockclassificationCategoryStore(t)
			tags := newMockclassificationTagStore(t)
			categoryID := "category-" + fake.UUID().V4()
			tagID := "tag-" + fake.UUID().V4()
			transaction := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "match"}
			rules.EXPECT().ListClassificationRules(t.Context(), params.TenantID, "").Return(
				[]domain.ClassificationRule{{
					ID:         "rule-" + fake.UUID().V4(),
					MatchType:  domain.ClassificationMatchTypeExact,
					Condition:  transaction.Description,
					CategoryID: categoryID,
					TagIDs:     []string{tagID},
				}},
				nil,
			).Once()
			transactions.EXPECT().ListEligibleClassificationTransactions(
				t.Context(),
				persistence.ListEligibleClassificationTransactionsParams{
					TenantID:          params.TenantID,
					RangeStart:        params.RangeStart,
					RangeEndExclusive: params.RangeEndExclusive,
				},
			).Return([]domain.Transaction{transaction}, nil).Once()
			categories.EXPECT().GetCategory(t.Context(), categoryID).Return(
				&domain.Category{ID: categoryID, TenantID: params.TenantID},
				nil,
			).Once()
			service, err := NewClassificationService(ClassificationServiceArgs{
				Access:       newMockaccessGuardStore(t),
				Rules:        rules,
				Transactions: transactions,
				Categories:   categories,
				Tags:         tags,
				Logger:       slog.New(slog.DiscardHandler),
				Now:          func() time.Time { return now },
			})
			require.NoError(t, err)
			return service, rules, transactions, categories, tags, params, transaction, categoryID, tagID
		}
		for _, tc := range []struct {
			name     string
			tag      *domain.Tag
			err      error
			terminal bool
		}{
			{name: "missing", err: persistence.ErrTagNotFound, terminal: true},
			{name: "hidden", tag: &domain.Tag{HiddenAt: &time.Time{}}, terminal: true},
			{name: "other tenant", tag: &domain.Tag{TenantID: "tenant-other-" + fake.UUID().V4()}, terminal: true},
			{name: "operational", err: fmt.Errorf("get tag: %w", errors.New("database unavailable"))},
		} {
			t.Run(tc.name, func(t *testing.T) {
				service, _, _, _, tags, params, _, _, tagID := makeService(t)
				if tc.tag != nil {
					tc.tag.ID = tagID
					if tc.tag.TenantID == "" {
						tc.tag.TenantID = params.TenantID
					}
				}
				tags.EXPECT().GetTag(t.Context(), tagID).Return(tc.tag, tc.err).Once()
				_, err := service.Classify(t.Context(), params)
				require.Error(t, err)
				failure, terminal := TerminalFailureFrom(err)
				assert.Equal(t, tc.terminal, terminal)
				if terminal {
					assert.Equal(t, "classification_tag_unavailable", failure.Code)
				}
			})
		}
	})

	t.Run("enforces all required dependencies", func(t *testing.T) {
		rules := newMockclassificationRuleStore(t)
		transactions := newMockclassificationTransactionStore(t)
		categories := newMockclassificationCategoryStore(t)
		_, err := NewClassificationService(ClassificationServiceArgs{})
		require.ErrorContains(t, err, "access store is required")
		access := newMockaccessGuardStore(t)
		_, err = NewClassificationService(ClassificationServiceArgs{Access: access})
		require.ErrorContains(t, err, "rule store is required")
		_, err = NewClassificationService(ClassificationServiceArgs{Access: access, Rules: rules})
		require.ErrorContains(t, err, "transaction store is required")
		_, err = NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
		})
		require.ErrorContains(t, err, "category store is required")
		_, err = NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
			Categories:   categories,
		})
		require.ErrorContains(t, err, "tag store is required")
		_, err = NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
			Categories:   categories,
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
		})
		require.ErrorContains(t, err, "clock is required")
	})

	t.Run("retains counts when a later operation fails", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 18, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := ClassificationParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now.Add(-time.Hour), RangeEndExclusive: now,
		}
		makeService := func(t *testing.T) (*ClassificationService, *mockclassificationRuleStore, *mockclassificationTransactionStore, *mockclassificationCategoryStore) {
			t.Helper()
			rules := newMockclassificationRuleStore(t)
			transactions := newMockclassificationTransactionStore(t)
			categories := newMockclassificationCategoryStore(t)
			access := newMockaccessGuardStore(t)
			service, err := NewClassificationService(ClassificationServiceArgs{
				Access:       access,
				Rules:        rules,
				Transactions: transactions,
				Categories:   categories,
				Tags:         newMockclassificationTagStore(t),
				Logger:       slog.New(slog.DiscardHandler),
				Now:          func() time.Time { return now },
			})
			require.NoError(t, err)
			return service, rules, transactions, categories
		}
		makeRule := func(condition, categoryID string) domain.ClassificationRule {
			return domain.ClassificationRule{
				ID:         "rule-" + fake.UUID().V4(),
				MatchType:  domain.ClassificationMatchTypeExact,
				Condition:  condition,
				CategoryID: categoryID,
			}
		}

		t.Run("reports rule load failures", func(t *testing.T) {
			service, rules, _, _ := makeService(t)
			loadErr := errors.New("load failed")
			rules.EXPECT().ListClassificationRules(t.Context(), params.TenantID, "").Return(nil, loadErr).Once()
			counts, err := service.Classify(t.Context(), params)
			require.ErrorIs(t, err, loadErr)
			assert.Equal(t, ClassificationAttemptCounts{}, counts)
		})

		t.Run("reports selection failures", func(t *testing.T) {
			service, rules, transactions, _ := makeService(t)
			listErr := errors.New("selection failed")
			rules.EXPECT().ListClassificationRules(t.Context(), params.TenantID, "").Return(nil, nil).Once()
			transactions.EXPECT().
				ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
					TenantID:          params.TenantID,
					RangeStart:        params.RangeStart,
					RangeEndExclusive: params.RangeEndExclusive,
				}).
				Return(nil, listErr).
				Once()
			counts, err := service.Classify(t.Context(), params)
			require.ErrorIs(t, err, listErr)
			assert.Equal(t, ClassificationAttemptCounts{}, counts)
		})

		t.Run("reports unavailable category states as terminal after prior committed assignments", func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				category func(categoryID string) (*domain.Category, error)
				cause    error
			}{
				{
					name: "persistence not found",
					category: func(string) (*domain.Category, error) {
						return nil, persistence.ErrCategoryNotFound
					},
					cause: persistence.ErrCategoryNotFound,
				},
				{
					name: "nil category",
					category: func(string) (*domain.Category, error) {
						return nil, nil //nolint:nilnil // The store contract permits a nil category without an error.
					},
				},
				{
					name: "hidden category",
					category: func(categoryID string) (*domain.Category, error) {
						hiddenAt := now
						return &domain.Category{ID: categoryID, TenantID: params.TenantID, HiddenAt: &hiddenAt}, nil
					},
				},
				{
					name: "cross tenant category",
					category: func(categoryID string) (*domain.Category, error) {
						return &domain.Category{ID: categoryID, TenantID: "tenant-" + fake.UUID().V4()}, nil
					},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					service, rules, transactions, categories := makeService(t)
					firstCategoryID := "category-" + fake.UUID().V4()
					secondCategoryID := "category-" + fake.UUID().V4()
					rules.EXPECT().
						ListClassificationRules(t.Context(), params.TenantID, "").
						Return([]domain.ClassificationRule{
							makeRule("first", firstCategoryID),
							makeRule("second", secondCategoryID),
						}, nil).
						Once()
					first := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "first"}
					second := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "second"}
					transactions.EXPECT().
						ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
							TenantID:          params.TenantID,
							RangeStart:        params.RangeStart,
							RangeEndExclusive: params.RangeEndExclusive,
						}).
						Return([]domain.Transaction{first, second}, nil).
						Once()
					categories.EXPECT().
						GetCategory(t.Context(), firstCategoryID).
						Return(&domain.Category{ID: firstCategoryID, TenantID: params.TenantID}, nil).
						Once()
					transactions.EXPECT().
						AssignClassification(t.Context(), persistence.AssignClassificationParams{
							TenantID:      params.TenantID,
							TransactionID: first.ID,
							CategoryID:    firstCategoryID,
							UpdatedAt:     now,
						}).
						Return(true, nil).
						Once()
					category, categoryErr := tc.category(secondCategoryID)
					categories.EXPECT().
						GetCategory(t.Context(), secondCategoryID).
						Return(category, categoryErr).
						Once()

					counts, err := service.Classify(t.Context(), params)
					require.Error(t, err)
					_, terminal := TerminalFailureFrom(err)
					assert.True(t, terminal)
					if tc.cause != nil {
						require.ErrorIs(t, err, tc.cause)
					}
					assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, counts)
				})
			}
		})

		t.Run("returns a transient category lookup error after prior committed assignments", func(t *testing.T) {
			service, rules, transactions, categories := makeService(t)
			firstCategoryID := "category-" + fake.UUID().V4()
			secondCategoryID := "category-" + fake.UUID().V4()
			rules.EXPECT().
				ListClassificationRules(t.Context(), params.TenantID, "").
				Return([]domain.ClassificationRule{
					makeRule("first", firstCategoryID),
					makeRule("second", secondCategoryID),
				}, nil).
				Once()
			first := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "first"}
			second := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "second"}
			transactions.EXPECT().
				ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
					TenantID:          params.TenantID,
					RangeStart:        params.RangeStart,
					RangeEndExclusive: params.RangeEndExclusive,
				}).
				Return([]domain.Transaction{first, second}, nil).
				Once()
			categories.EXPECT().
				GetCategory(t.Context(), firstCategoryID).
				Return(&domain.Category{ID: firstCategoryID, TenantID: params.TenantID}, nil).
				Once()
			transactions.EXPECT().
				AssignClassification(t.Context(), persistence.AssignClassificationParams{
					TenantID: params.TenantID, TransactionID: first.ID, CategoryID: firstCategoryID, UpdatedAt: now,
				}).
				Return(true, nil).
				Once()
			lookupCause := errors.New("database unavailable")
			lookupErr := fmt.Errorf("load category: %w", lookupCause)
			categories.EXPECT().
				GetCategory(t.Context(), secondCategoryID).
				Return(nil, lookupErr).
				Once()

			counts, err := service.Classify(t.Context(), params)
			require.ErrorIs(t, err, lookupCause)
			_, terminal := TerminalFailureFrom(err)
			assert.False(t, terminal)
			assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, counts)
		})

		t.Run("counts a conditional-write race as skipped", func(t *testing.T) {
			service, rules, transactions, categories := makeService(t)
			categoryID := "category-" + fake.UUID().V4()
			rules.EXPECT().
				ListClassificationRules(t.Context(), params.TenantID, "").
				Return([]domain.ClassificationRule{
					{
						ID: "rule-" + fake.UUID().
							V4(),
						MatchType:  domain.ClassificationMatchTypeExact,
						Condition:  "match",
						CategoryID: categoryID,
					},
				}, nil).
				Once()
			transaction := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: "match"}
			transactions.EXPECT().
				ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
					TenantID:          params.TenantID,
					RangeStart:        params.RangeStart,
					RangeEndExclusive: params.RangeEndExclusive,
				}).
				Return([]domain.Transaction{transaction}, nil).
				Once()
			categories.EXPECT().
				GetCategory(t.Context(), categoryID).
				Return(&domain.Category{ID: categoryID, TenantID: params.TenantID}, nil).
				Once()
			transactions.EXPECT().
				AssignClassification(t.Context(), persistence.AssignClassificationParams{
					TenantID: params.TenantID, TransactionID: transaction.ID, CategoryID: categoryID, UpdatedAt: now,
				}).
				Return(false, nil).
				Once()
			transactions.EXPECT().
				ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
					TenantID:          params.TenantID,
					RangeStart:        params.RangeStart,
					RangeEndExclusive: params.RangeEndExclusive,
					AfterID:           transaction.ID,
				}).
				Return(nil, nil).
				Once()
			counts, err := service.Classify(t.Context(), params)
			require.NoError(t, err)
			assert.Equal(t, ClassificationAttemptCounts{Skipped: 1}, counts)
		})
	})

	t.Run("classifies newly eligible same-range rows without overwriting earlier assignments", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		ruleStore := persistence.NewClassificationRuleStore(database)
		transactionStore := persistence.NewClassificationTransactionStore(database)
		now := time.Date(2026, time.September, 7, 11, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		condition := "merchant-" + fake.Lorem().Word()
		_, err := store.SaveCategory(t.Context(), domain.Category{
			ID: categoryID, TenantID: tenantID, Name: "category-" + fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		_, err = ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: tenantID,
			MatchType: domain.ClassificationMatchTypeExact, Condition: condition, CategoryID: categoryID,
			CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		makeTransaction := func() domain.Transaction {
			return domain.Transaction{
				ID: "transaction-" + fake.UUID().V4(), TenantID: tenantID, AccountID: "account-" + fake.UUID().V4(),
				Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindExpense, AmountMinor: -1, Currency: "USD", Description: condition,
				EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
			}
		}
		firstTransaction := makeTransaction()
		_, err = store.SaveTransaction(t.Context(), firstTransaction)
		require.NoError(t, err)
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access: store, Rules: ruleStore, Transactions: transactionStore, Categories: store, Tags: store,
			Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
		})
		require.NoError(t, err)
		params := ClassificationParams{
			TenantID: tenantID, RangeStart: now.Add(-time.Hour), RangeEndExclusive: now.Add(time.Hour),
			MessageID: "message-" + fake.UUID().V4(),
		}
		firstCounts, err := service.Classify(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, firstCounts)

		secondTransaction := makeTransaction()
		_, err = store.SaveTransaction(t.Context(), secondTransaction)
		require.NoError(t, err)
		params.MessageID = "message-" + fake.UUID().V4()
		secondCounts, err := service.Classify(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, secondCounts)
		for _, transactionID := range []string{firstTransaction.ID, secondTransaction.ID} {
			stored, getErr := store.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			require.NotNil(t, stored.CategoryID)
			assert.Equal(t, categoryID, *stored.CategoryID)
		}
		remaining, err := transactionStore.ListEligibleClassificationTransactions(
			t.Context(),
			persistence.ListEligibleClassificationTransactionsParams{
				TenantID: tenantID, RangeStart: now.Add(-time.Hour), RangeEndExclusive: now.Add(time.Hour),
			},
		)
		require.NoError(t, err)
		assert.Empty(t, remaining)
	})

	t.Run("reloads current rules to classify new same-range rows while excluding categorized rows", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 11, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := ClassificationParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now.Add(-time.Hour), RangeEndExclusive: now,
		}
		rules := newMockclassificationRuleStore(t)
		transactions := newMockclassificationTransactionStore(t)
		categories := newMockclassificationCategoryStore(t)
		access := newMockaccessGuardStore(t)
		service, err := NewClassificationService(ClassificationServiceArgs{
			Access:       access,
			Rules:        rules,
			Transactions: transactions,
			Categories:   categories,
			Tags:         newMockclassificationTagStore(t),
			Logger:       slog.New(slog.DiscardHandler),
			Now:          func() time.Time { return now },
		})
		require.NoError(t, err)
		categoryID := "category-" + fake.UUID().V4()
		firstRule := domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), MatchType: domain.ClassificationMatchTypeExact,
			Condition: "first", CategoryID: categoryID,
		}
		secondRule := domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), MatchType: domain.ClassificationMatchTypeExact,
			Condition: "changed", CategoryID: categoryID,
		}
		transaction := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: firstRule.Condition}
		newTransaction := domain.Transaction{ID: "transaction-" + fake.UUID().V4(), Description: secondRule.Condition}
		rules.EXPECT().
			ListClassificationRules(t.Context(), params.TenantID, "").
			Return([]domain.ClassificationRule{firstRule}, nil).
			Once()
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID: params.TenantID, RangeStart: params.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
			}).
			Return([]domain.Transaction{transaction}, nil).
			Once()
		categories.EXPECT().
			GetCategory(t.Context(), categoryID).
			Return(&domain.Category{ID: categoryID, TenantID: params.TenantID}, nil).
			Once()
		transactions.EXPECT().AssignClassification(t.Context(), persistence.AssignClassificationParams{
			TenantID: params.TenantID, TransactionID: transaction.ID, CategoryID: categoryID, UpdatedAt: now,
		}).Return(true, nil).Once()
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          params.TenantID,
				RangeStart:        params.RangeStart,
				RangeEndExclusive: params.RangeEndExclusive,
				AfterID:           transaction.ID,
			}).
			Return(nil, nil).
			Once()
		firstCounts, err := service.Classify(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, firstCounts)

		rules.EXPECT().
			ListClassificationRules(t.Context(), params.TenantID, "").
			Return([]domain.ClassificationRule{secondRule}, nil).
			Once()
		secondParams := params
		secondParams.MessageID = "message-" + fake.UUID().V4()
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          secondParams.TenantID,
				RangeStart:        secondParams.RangeStart,
				RangeEndExclusive: secondParams.RangeEndExclusive,
			}).
			Return([]domain.Transaction{newTransaction}, nil).
			Once()
		categories.EXPECT().
			GetCategory(t.Context(), categoryID).
			Return(&domain.Category{ID: categoryID, TenantID: secondParams.TenantID}, nil).
			Once()
		transactions.EXPECT().AssignClassification(t.Context(), persistence.AssignClassificationParams{
			TenantID: secondParams.TenantID, TransactionID: newTransaction.ID, CategoryID: categoryID, UpdatedAt: now,
		}).Return(true, nil).Once()
		transactions.EXPECT().
			ListEligibleClassificationTransactions(t.Context(), persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          secondParams.TenantID,
				RangeStart:        secondParams.RangeStart,
				RangeEndExclusive: secondParams.RangeEndExclusive,
				AfterID:           newTransaction.ID,
			}).
			Return(nil, nil).
			Once()
		secondCounts, err := service.Classify(t.Context(), secondParams)
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, secondCounts)
	})
}
