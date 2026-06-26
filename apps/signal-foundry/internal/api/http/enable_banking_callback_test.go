package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEnableBankingCallbackLookup struct {
	start domain.PendingBankConnectionLinkStart
	err   error
}

func (s stubEnableBankingCallbackLookup) GetPendingBankConnectionLinkStartByState(
	context.Context,
	financepkg.GetPendingBankConnectionLinkStartByStateParams,
) (domain.PendingBankConnectionLinkStart, error) {
	if s.err != nil {
		return domain.PendingBankConnectionLinkStart{}, s.err
	}
	return s.start, nil
}

func TestEnableBankingCallbackHandler(t *testing.T) {
	t.Run("redirects callback params to the stored browser route", func(t *testing.T) {
		handler := newEnableBankingCallbackHandler(
			stubEnableBankingCallbackLookup{start: domain.PendingBankConnectionLinkStart{
				CallbackURL: "http://localhost:5173/#/finance/connections",
			}},
		)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state=state-1",
				http.NoBody,
			),
		)
		require.Equal(t, http.StatusFound, resp.Code)
		assert.Equal(
			t,
			"http://localhost:5173/?code=code-1&state=state-1#/finance/connections",
			resp.Header().Get("Location"),
		)
	})

	t.Run("rejects missing state and invalid callback targets", func(t *testing.T) {
		missingStateResp := httptest.NewRecorder()
		newEnableBankingCallbackHandler(stubEnableBankingCallbackLookup{}).ServeHTTP(
			missingStateResp,
			httptest.NewRequest(http.MethodGet, "/enable-banking/callback?code=code-1", http.NoBody),
		)
		require.Equal(t, http.StatusBadRequest, missingStateResp.Code)

		invalidCallbackResp := httptest.NewRecorder()
		newEnableBankingCallbackHandler(
			stubEnableBankingCallbackLookup{start: domain.PendingBankConnectionLinkStart{
				CallbackURL: "https://app.example.test/#/finance/other",
			}},
		).ServeHTTP(
			invalidCallbackResp,
			httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state=state-1",
				http.NoBody,
			),
		)
		require.Equal(t, http.StatusBadRequest, invalidCallbackResp.Code)
	})

	t.Run("maps lookup failures to callback resolution errors", func(t *testing.T) {
		notFoundResp := httptest.NewRecorder()
		newEnableBankingCallbackHandler(
			stubEnableBankingCallbackLookup{err: financepkg.ErrPendingBankConnectionLinkStartNotFound},
		).ServeHTTP(
			notFoundResp,
			httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state=state-1",
				http.NoBody,
			),
		)
		require.Equal(t, http.StatusBadRequest, notFoundResp.Code)

		unexpectedResp := httptest.NewRecorder()
		newEnableBankingCallbackHandler(
			stubEnableBankingCallbackLookup{err: errors.New("boom")},
		).ServeHTTP(
			unexpectedResp,
			httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state=state-1",
				http.NoBody,
			),
		)
		require.Equal(t, http.StatusInternalServerError, unexpectedResp.Code)
	})
}

func TestBuildEnableBankingBrowserHandoffURL(t *testing.T) {
	handoffURL, err := buildEnableBankingBrowserHandoffURL(
		"http://localhost:5173/#/finance/connections",
		url.Values{"code": []string{"code-1"}, "state": []string{"state-1"}},
	)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5173/?code=code-1&state=state-1#/finance/connections", handoffURL)

	_, err = buildEnableBankingBrowserHandoffURL("://bad-url", url.Values{})
	require.Error(t, err)
}
