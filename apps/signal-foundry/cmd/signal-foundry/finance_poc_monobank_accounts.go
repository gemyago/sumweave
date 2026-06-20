package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type monobankCommandParams struct {
	JSON       bool
	OutputFile string
	Timeout    time.Duration
	BaseURL    string
	Token      string
}

type monobankAccountsOutput struct {
	Provider       string                       `json:"provider"`
	Operation      string                       `json:"operation"`
	FetchedAt      string                       `json:"fetchedAt"`
	Name           string                       `json:"name,omitempty"`
	Accounts       []monobankAccountEntry       `json:"accounts"`
	Jars           []monobankJarEntry           `json:"jars"`
	ManagedClients []monobankManagedClientEntry `json:"managedClients"`
	Raw            map[string]any               `json:"raw"`
}

type monobankAccountEntry struct {
	ID           string         `json:"id"`
	Type         string         `json:"type,omitempty"`
	CurrencyCode int            `json:"currencyCode,omitempty"`
	Balance      int64          `json:"balance,omitempty"`
	CreditLimit  int64          `json:"creditLimit,omitempty"`
	MaskedPAN    []string       `json:"maskedPan,omitempty"`
	IBAN         string         `json:"iban,omitempty"`
	Raw          map[string]any `json:"raw"`
}

type monobankJarEntry struct {
	ID           string         `json:"id"`
	CurrencyCode int            `json:"currencyCode,omitempty"`
	Balance      int64          `json:"balance,omitempty"`
	IBAN         string         `json:"iban,omitempty"`
	Raw          map[string]any `json:"raw"`
}

type monobankManagedClientEntry struct {
	ClientID string                 `json:"clientId,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Accounts []monobankAccountEntry `json:"accounts"`
	Raw      map[string]any         `json:"raw"`
}

func newMonobankAccountsCmd(deps financePOCCommandDeps, requestParams *monobankCommandParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   monobankAccountsOp,
		Short: "List monobank accounts, jars, and managed clients",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := financePOCProviderRequest{
				Provider:  monobankCommandName,
				Operation: monobankAccountsOp,
				BaseURL: firstNonEmpty(
					strings.TrimSpace(requestParams.BaseURL),
					strings.TrimSpace(cmd.Flag("base-url").Value.String()),
					financePOCEnv("MONOBANK_BASE_URL"),
					monobankDefaultBaseURL,
				),
				Timeout:    requestParams.Timeout,
				JSON:       requestParams.JSON,
				OutputFile: strings.TrimSpace(requestParams.OutputFile),
				Token: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.Token),
					financePOCEnv("MONOBANK_TOKEN"),
				),
				TokenSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.Token),
					financePOCEnv("MONOBANK_TOKEN"),
				),
			}

			result, err := runMonobankAccounts(cmd.Context(), cmd.ErrOrStderr(), deps, request)
			if err != nil {
				return err
			}

			return writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				request.OutputFile,
				request.JSON,
				result,
				fmt.Sprintf(
					"accounts: %d\njars: %d\nmanaged_clients: %d\n",
					len(result.Accounts),
					len(result.Jars),
					len(result.ManagedClients),
				),
			)
		},
	}
	return cmd
}

func runMonobankAccounts(
	ctx context.Context,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
) (monobankAccountsOutput, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if strings.TrimSpace(request.Token) == "" {
		return monobankAccountsOutput{}, errors.New("monobank token is required")
	}

	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()

	raw, err := callMonobankClientInfo(ctx, request)
	if err != nil {
		return monobankAccountsOutput{}, err
	}

	accounts := normalizeMonobankAccounts(extractMonobankObjects(raw, "accounts"))
	jars := normalizeMonobankJars(extractMonobankObjects(raw, "jars"))
	managedClients := normalizeMonobankManagedClients(raw)

	writeFinancePOCProgressf(
		stderr,
		"retrieved %d monobank accounts, %d jars, %d managed clients",
		len(accounts),
		len(jars),
		len(managedClients),
	)

	return monobankAccountsOutput{
		Provider:       monobankCommandName,
		Operation:      monobankAccountsOp,
		FetchedAt:      deps.Now().UTC().Format(time.RFC3339),
		Name:           extractMonobankString(raw, "name"),
		Accounts:       accounts,
		Jars:           jars,
		ManagedClients: managedClients,
		Raw:            raw,
	}, nil
}

func callMonobankClientInfo(ctx context.Context, request financePOCProviderRequest) (map[string]any, error) {
	endpoint, err := url.Parse(strings.TrimRight(request.BaseURL, "/") + "/personal/client-info")
	if err != nil {
		return nil, fmt.Errorf("parse monobank client-info URL: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build monobank client-info request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Token", request.Token)

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("request monobank client-info: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read monobank client-info response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, newFinancePOCProviderResponseError(
			monobankCommandName,
			monobankAccountsOp,
			response.StatusCode,
			body,
		)
	}

	var raw map[string]any
	if unmarshalErr := json.Unmarshal(body, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("decode monobank client-info response: %w", unmarshalErr)
	}
	return raw, nil
}

func normalizeMonobankAccounts(items []map[string]any) []monobankAccountEntry {
	accounts := make([]monobankAccountEntry, 0, len(items))
	for _, item := range items {
		accounts = append(accounts, monobankAccountEntry{
			ID:           extractMonobankString(item, "id"),
			Type:         extractMonobankString(item, "type"),
			CurrencyCode: extractMonobankInt(item, "currencyCode"),
			Balance:      extractMonobankInt64(item, "balance"),
			CreditLimit:  extractMonobankInt64(item, "creditLimit"),
			MaskedPAN:    extractMonobankMaskedPANs(item),
			IBAN:         extractMonobankString(item, "iban"),
			Raw:          item,
		})
	}
	return accounts
}

func normalizeMonobankJars(items []map[string]any) []monobankJarEntry {
	jars := make([]monobankJarEntry, 0, len(items))
	for _, item := range items {
		jars = append(jars, monobankJarEntry{
			ID:           extractMonobankString(item, "id"),
			CurrencyCode: extractMonobankInt(item, "currencyCode"),
			Balance:      extractMonobankInt64(item, "balance"),
			IBAN:         extractMonobankString(item, "iban"),
			Raw:          item,
		})
	}
	return jars
}

func normalizeMonobankManagedClients(raw map[string]any) []monobankManagedClientEntry {
	items := extractMonobankObjects(raw, "clients")
	if len(items) == 0 {
		items = extractMonobankObjects(raw, "managedClients")
	}

	clients := make([]monobankManagedClientEntry, 0, len(items))
	for _, item := range items {
		clients = append(clients, monobankManagedClientEntry{
			ClientID: extractMonobankString(item, "clientId", "id"),
			Name:     extractMonobankString(item, "name"),
			Accounts: normalizeMonobankAccounts(extractMonobankObjects(item, "accounts")),
			Raw:      item,
		})
	}
	return clients
}

func extractMonobankObjects(raw map[string]any, key string) []map[string]any {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	items, itemsOK := value.([]any)
	if !itemsOK {
		return nil
	}
	objects := make([]map[string]any, 0, len(items))
	for _, item := range items {
		object, objectOK := item.(map[string]any)
		if !objectOK {
			continue
		}
		objects = append(objects, object)
	}
	return objects
}

func extractMonobankString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		stringValue, ok := value.(string)
		if ok {
			return strings.TrimSpace(stringValue)
		}
	}
	return ""
}

func extractMonobankMaskedPANs(raw map[string]any) []string {
	value, ok := raw["maskedPan"]
	if !ok {
		return nil
	}
	items, itemsOK := value.([]any)
	if !itemsOK {
		return nil
	}
	stringsValue := make([]string, 0, len(items))
	for _, item := range items {
		stringValue, stringOK := item.(string)
		if !stringOK {
			continue
		}
		stringsValue = append(stringsValue, strings.TrimSpace(stringValue))
	}
	if len(stringsValue) == 0 {
		return nil
	}
	return stringsValue
}

func extractMonobankInt(raw map[string]any, key string) int {
	return int(extractMonobankInt64(raw, key))
}

func extractMonobankInt64(raw map[string]any, key string) int64 {
	value, ok := raw[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	}
	return 0
}
