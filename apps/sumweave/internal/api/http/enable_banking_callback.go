package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1controllers"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
)

const enableBankingCallbackPath = "/enable-banking/callback"

type enableBankingCallbackLookup interface {
	GetPendingBankConnectionLinkStartByState(
		ctx context.Context,
		params financepkg.GetPendingBankConnectionLinkStartByStateParams,
	) (domain.PendingBankConnectionLinkStart, error)
}

func newEnableBankingCallbackHandler(lookup enableBankingCallbackLookup) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		state := strings.TrimSpace(req.URL.Query().Get("state"))
		if state == "" {
			http.Error(w, "missing state", http.StatusBadRequest)
			return
		}
		pendingStart, err := lookup.GetPendingBankConnectionLinkStartByState(
			req.Context(),
			financepkg.GetPendingBankConnectionLinkStartByStateParams{Provider: "pko", State: state},
		)
		if err != nil {
			status := http.StatusBadRequest
			if !errors.Is(err, financepkg.ErrPendingBankConnectionLinkStartNotFound) {
				status = http.StatusInternalServerError
			}
			http.Error(w, "unable to resolve callback target", status)
			return
		}
		if callbackErr := v1controllers.ValidateFinanceRedirectCallbackURL(
			pendingStart.CallbackURL,
		); callbackErr != nil {
			http.Error(w, "invalid callback target", http.StatusBadRequest)
			return
		}

		handoffURL, err := buildEnableBankingBrowserHandoffURL(pendingStart.CallbackURL, req.URL.Query())
		if err != nil {
			http.Error(w, "invalid callback redirect", http.StatusBadRequest)
			return
		}

		http.Redirect(w, req, handoffURL, http.StatusFound)
	})
}

func buildEnableBankingBrowserHandoffURL(callbackURL string, values url.Values) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, items := range values {
		query.Del(key)
		for _, item := range items {
			query.Add(key, item)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
