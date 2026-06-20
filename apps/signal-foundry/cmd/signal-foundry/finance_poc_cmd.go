package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

const (
	financePOCCommandName        = "finance-poc"
	enableBankingCommandName     = "enable-banking"
	monobankCommandName          = "monobank"
	financePOCOperationStatus    = "status"
	financePOCSummaryBaseURLKey  = "baseURL"
	enableBankingDefaultBaseURL  = "https://api.enablebanking.com"
	enableBankingJWTIssuer       = "enablebanking.com"
	enableBankingJWTAudience     = "api.enablebanking.com"
	enableBankingASPSPsOperation = "aspsps"
	enableBankingStartAuthOp     = "start-auth"
	enableBankingFinishSessionOp = "finish-session"
	enableBankingConnectOp       = "connect"
	enableBankingAccountsOp      = "accounts"
	enableBankingTransactionsOp  = "transactions"
	monobankAccountsOp           = "accounts"
	monobankTransactionsOp       = "transactions"
	monobankDefaultBaseURL       = "https://api.monobank.ua"
	financePOCTokenSourceEnv     = "env"
	financePOCTokenSourceFlag    = "flag"
	financePOCTokenSourceNone    = "none"
	financePOCAppIDSourceEnv     = "env"
	financePOCAppIDSourceFlag    = "flag"
	financePOCAppIDSourceNone    = "none"
	financePOCKeyPathSourceEnv   = "env"
	financePOCKeyPathSourceFlag  = "flag"
	financePOCKeyPathSourceNone  = "none"
)

const enableBankingJWTLifetime = 5 * time.Minute

type financePOCProviderRunner func(context.Context, financePOCProviderRequest) (financePOCProviderResult, error)

type financePOCCommandDeps struct {
	Now                      func() time.Time
	Sleep                    func(time.Duration)
	EnableBankingRunner      financePOCProviderRunner
	MonobankRunner           financePOCProviderRunner
	EnableBankingState       func() (string, error)
	EnableBankingOpenBrowser func(string) error
}

type financePOCProviderRequest struct {
	Provider       string
	Operation      string
	Country        string
	BaseURL        string
	Timeout        time.Duration
	JSON           bool
	OutputFile     string
	Token          string
	TokenSource    string
	AppID          string
	AppIDSource    string
	PrivateKeyPath string
	KeyPathSource  string
}

type financePOCProviderResult struct {
	Summary map[string]any
	Raw     any
}

type financePOCEnvelope struct {
	Provider  string         `json:"provider"`
	Operation string         `json:"operation"`
	FetchedAt string         `json:"fetchedAt"`
	Summary   map[string]any `json:"summary"`
	Raw       any            `json:"raw,omitempty"`
}

type enableBankingCommandParams struct {
	JSON           bool
	OutputFile     string
	Timeout        time.Duration
	BaseURL        string
	AppID          string
	PrivateKeyPath string
}

func newFinancePOCCmd(deps financePOCCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   financePOCCommandName,
		Short: "Run isolated financial POC commands",
	}
	cmd.AddCommand(
		newEnableBankingCmd(deps),
		newMonobankCmd(deps),
	)
	return cmd
}

func newEnableBankingCmd(deps financePOCCommandDeps) *cobra.Command {
	requestParams := enableBankingCommandParams{Timeout: 30 * time.Second}
	cmd := &cobra.Command{
		Use:   enableBankingCommandName,
		Short: "Enable Banking POC command skeleton",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := financePOCProviderRequest{
				Provider: enableBankingCommandName,
				BaseURL: firstNonEmpty(
					strings.TrimSpace(requestParams.BaseURL),
					strings.TrimSpace(cmd.Flag("base-url").Value.String()),
					financePOCEnv("ENABLE_BANKING_BASE_URL"),
					enableBankingDefaultBaseURL,
				),
				Timeout:    requestParams.Timeout,
				JSON:       requestParams.JSON,
				OutputFile: strings.TrimSpace(requestParams.OutputFile),
				AppID: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.AppID),
					financePOCEnv("ENABLE_BANKING_APP_ID"),
				),
				AppIDSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.AppID),
					financePOCEnv("ENABLE_BANKING_APP_ID"),
				),
				PrivateKeyPath: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.PrivateKeyPath),
					financePOCEnv("ENABLE_BANKING_PRIVATE_KEY_PATH"),
				),
				KeyPathSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.PrivateKeyPath),
					financePOCEnv("ENABLE_BANKING_PRIVATE_KEY_PATH"),
				),
			}

			return runFinancePOCProviderStatus(
				cmd.Context(),
				cmd.OutOrStdout(),
				cmd.ErrOrStderr(),
				deps,
				request,
				deps.EnableBankingRunner,
			)
		},
	}
	cmd.PersistentFlags().BoolVar(&requestParams.JSON, "json", false, "Print machine-readable JSON to stdout")
	cmd.PersistentFlags().StringVar(&requestParams.OutputFile, "out", "", "Optional output file path")
	cmd.PersistentFlags().DurationVar(&requestParams.Timeout, "timeout", requestParams.Timeout, "Request timeout")
	cmd.PersistentFlags().StringVar(&requestParams.BaseURL, "base-url", "", "Override Enable Banking base URL")
	cmd.PersistentFlags().StringVar(&requestParams.AppID, "app-id", "", "Override Enable Banking app ID")
	cmd.PersistentFlags().StringVar(
		&requestParams.PrivateKeyPath,
		"private-key-path",
		"",
		"Override Enable Banking private key path",
	)
	cmd.AddCommand(
		newEnableBankingASPSPsCmd(deps, &requestParams),
		newEnableBankingStartAuthCmd(deps, &requestParams),
		newEnableBankingFinishSessionCmd(deps, &requestParams),
		newEnableBankingConnectCmd(deps, &requestParams),
		newEnableBankingAccountsCmd(deps, &requestParams),
		newEnableBankingTransactionsCmd(deps, &requestParams),
	)
	return cmd
}

func newEnableBankingASPSPsCmd(deps financePOCCommandDeps, requestParams *enableBankingCommandParams) *cobra.Command {
	country := ""
	cmd := &cobra.Command{
		Use:   enableBankingASPSPsOperation,
		Short: "List available ASPSPs for a country",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := financePOCProviderRequest{
				Provider:  enableBankingCommandName,
				Operation: enableBankingASPSPsOperation,
				Country:   strings.TrimSpace(country),
				BaseURL: firstNonEmpty(
					strings.TrimSpace(requestParams.BaseURL),
					strings.TrimSpace(cmd.Flag("base-url").Value.String()),
					financePOCEnv("ENABLE_BANKING_BASE_URL"),
					enableBankingDefaultBaseURL,
				),
				Timeout:    requestParams.Timeout,
				JSON:       requestParams.JSON,
				OutputFile: strings.TrimSpace(requestParams.OutputFile),
				AppID: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.AppID),
					financePOCEnv("ENABLE_BANKING_APP_ID"),
				),
				AppIDSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.AppID),
					financePOCEnv("ENABLE_BANKING_APP_ID"),
				),
				PrivateKeyPath: resolveFinancePOCStringSetting(
					strings.TrimSpace(requestParams.PrivateKeyPath),
					financePOCEnv("ENABLE_BANKING_PRIVATE_KEY_PATH"),
				),
				KeyPathSource: resolveFinancePOCStringSource(
					strings.TrimSpace(requestParams.PrivateKeyPath),
					financePOCEnv("ENABLE_BANKING_PRIVATE_KEY_PATH"),
				),
			}

			return runFinancePOCProviderStatus(
				cmd.Context(),
				cmd.OutOrStdout(),
				cmd.ErrOrStderr(),
				deps,
				request,
				deps.EnableBankingRunner,
			)
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "Two-letter country code")
	_ = cmd.MarkFlagRequired("country")
	return cmd
}

func newMonobankCmd(deps financePOCCommandDeps) *cobra.Command {
	requestParams := monobankCommandParams{Timeout: 30 * time.Second}
	cmd := &cobra.Command{
		Use:   monobankCommandName,
		Short: "monobank POC command skeleton",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := financePOCProviderRequest{
				Provider: monobankCommandName,
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

			return runFinancePOCProviderStatus(
				cmd.Context(),
				cmd.OutOrStdout(),
				cmd.ErrOrStderr(),
				deps,
				request,
				deps.MonobankRunner,
			)
		},
	}
	cmd.PersistentFlags().BoolVar(&requestParams.JSON, "json", false, "Print machine-readable JSON to stdout")
	cmd.PersistentFlags().StringVar(&requestParams.OutputFile, "out", "", "Optional output file path")
	cmd.PersistentFlags().DurationVar(&requestParams.Timeout, "timeout", requestParams.Timeout, "Request timeout")
	cmd.PersistentFlags().StringVar(&requestParams.BaseURL, "base-url", "", "Override monobank base URL")
	cmd.PersistentFlags().StringVar(&requestParams.Token, "token", "", "Override monobank token")
	cmd.AddCommand(
		newMonobankAccountsCmd(deps, &requestParams),
		newMonobankTransactionsCmd(deps, &requestParams),
	)
	return cmd
}

func runFinancePOCProviderStatus(
	ctx context.Context,
	stdout io.Writer,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	runner financePOCProviderRunner,
) error {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if runner == nil {
		runner = func(ctx context.Context, request financePOCProviderRequest) (financePOCProviderResult, error) {
			return defaultFinancePOCProviderRunner(ctx, request, deps.Now)
		}
	}

	writeFinancePOCProgressf(stderr, "resolved finance-poc provider configuration for %s", request.Provider)
	result, err := runFinancePOCProviderWithTimeout(ctx, request, runner)
	if err != nil {
		return err
	}

	envelope := financePOCEnvelope{
		Provider:  request.Provider,
		Operation: firstNonEmpty(request.Operation, financePOCOperationStatus),
		FetchedAt: deps.Now().UTC().Format(time.RFC3339),
		Summary:   result.Summary,
		Raw:       result.Raw,
	}

	return writeFinancePOCEnvelope(stdout, request.OutputFile, request.JSON, envelope)
}

func runFinancePOCProviderWithTimeout(
	parent context.Context,
	request financePOCProviderRequest,
	runner financePOCProviderRunner,
) (financePOCProviderResult, error) {
	ctx := parent
	cancel := func() {}
	if request.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, request.Timeout)
	}
	defer cancel()

	result, err := runner(ctx, request)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return financePOCProviderResult{}, fmt.Errorf("%s timed out after %s", request.Provider, request.Timeout)
	}
	if err != nil {
		return financePOCProviderResult{}, errors.New(sanitizeFinancePOCText(err.Error()))
	}
	return result, nil
}

func defaultFinancePOCProviderRunner(
	ctx context.Context,
	request financePOCProviderRequest,
	now func() time.Time,
) (financePOCProviderResult, error) {
	if now == nil {
		now = time.Now
	}
	if request.Provider == enableBankingCommandName && request.Operation == enableBankingASPSPsOperation {
		return runEnableBankingASPSPs(ctx, request, now)
	}

	return financePOCProviderResult{
		Summary: map[string]any{
			financePOCSummaryBaseURLKey: request.BaseURL,
			"timeout":                   request.Timeout.String(),
			"tokenConfigured":           request.Token != "",
			"tokenSource":               request.TokenSource,
			"appIDConfigured":           request.AppID != "",
			"appIDSource":               request.AppIDSource,
			"privateKeyConfigured":      request.PrivateKeyPath != "",
			"privateKeySource":          request.KeyPathSource,
		},
	}, nil
}

func runEnableBankingASPSPs(
	ctx context.Context,
	request financePOCProviderRequest,
	now func() time.Time,
) (financePOCProviderResult, error) {
	if strings.TrimSpace(request.AppID) == "" {
		return financePOCProviderResult{}, errors.New("enable-banking app ID is required")
	}
	if strings.TrimSpace(request.PrivateKeyPath) == "" {
		return financePOCProviderResult{}, errors.New("enable-banking private key path is required")
	}
	if strings.TrimSpace(request.Country) == "" {
		return financePOCProviderResult{}, errors.New("enable-banking country is required")
	}

	accessToken, err := newEnableBankingJWT(now(), request.AppID, request.PrivateKeyPath)
	if err != nil {
		return financePOCProviderResult{}, err
	}

	endpoint, err := url.Parse(strings.TrimRight(request.BaseURL, "/") + "/aspsps")
	if err != nil {
		return financePOCProviderResult{}, fmt.Errorf("parse enable-banking aspsps URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("country", request.Country)
	endpoint.RawQuery = query.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return financePOCProviderResult{}, fmt.Errorf("build enable-banking aspsps request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	httpRequest.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return financePOCProviderResult{}, fmt.Errorf("request enable-banking aspsps: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return financePOCProviderResult{}, fmt.Errorf("read enable-banking aspsps response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return financePOCProviderResult{}, newFinancePOCProviderResponseError(
			enableBankingCommandName,
			enableBankingASPSPsOperation,
			response.StatusCode,
			body,
		)
	}

	var raw any
	if unmarshalErr := json.Unmarshal(body, &raw); unmarshalErr != nil {
		return financePOCProviderResult{}, fmt.Errorf("decode enable-banking aspsps response: %w", unmarshalErr)
	}

	count := 0
	if items, ok := raw.([]any); ok {
		count = len(items)
	}

	return financePOCProviderResult{
		Summary: map[string]any{
			financePOCSummaryBaseURLKey: strings.TrimRight(request.BaseURL, "/"),
			"country":                   request.Country,
			"count":                     count,
		},
		Raw: raw,
	}, nil
}

func newEnableBankingJWT(now time.Time, appID string, privateKeyPath string) (string, error) {
	privateKey, err := loadEnableBankingPrivateKey(privateKeyPath)
	if err != nil {
		return "", err
	}

	claims := jwt.RegisteredClaims{
		Issuer:    enableBankingJWTIssuer,
		Audience:  jwt.ClaimStrings{enableBankingJWTAudience},
		IssuedAt:  jwt.NewNumericDate(now.UTC()),
		ExpiresAt: jwt.NewNumericDate(now.UTC().Add(enableBankingJWTLifetime)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = strings.TrimSpace(appID)

	prevMarshalSingleStringAsArray := jwt.MarshalSingleStringAsArray
	jwt.MarshalSingleStringAsArray = false
	defer func() {
		jwt.MarshalSingleStringAsArray = prevMarshalSingleStringAsArray
	}()

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign enable-banking JWT: %w", err)
	}
	return signedToken, nil
}

func loadEnableBankingPrivateKey(privateKeyPath string) (any, error) {
	privateKeyPEM, err := os.ReadFile(strings.TrimSpace(privateKeyPath))
	if err != nil {
		return nil, fmt.Errorf("read enable-banking private key file: %w", err)
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse enable-banking private key file: %w", err)
	}
	return privateKey, nil
}

func resolveFinancePOCStringSetting(flagValue string, envValue string) string {
	return firstNonEmpty(strings.TrimSpace(flagValue), strings.TrimSpace(envValue))
}

func resolveFinancePOCStringSource(
	flagValue string,
	envValue string,
) string {
	if strings.TrimSpace(flagValue) != "" {
		return financePOCTokenSourceFlag
	}
	if strings.TrimSpace(envValue) != "" {
		return financePOCTokenSourceEnv
	}
	return financePOCTokenSourceNone
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func financePOCEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
