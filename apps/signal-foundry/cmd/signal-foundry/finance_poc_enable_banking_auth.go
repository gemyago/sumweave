package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	enableBankingPendingAuthKind = "pending-auth"
	enableBankingLocalhost       = "localhost"
	enableBankingPrivateKeyType  = "PRIVATE KEY"
)

type enableBankingPendingAuthRequest struct {
	Country     string `json:"country"`
	ASPSPName   string `json:"aspspName"`
	PSUType     string `json:"psuType"`
	ValidDays   int    `json:"validDays"`
	RedirectURL string `json:"redirectUrl"`
}

type enableBankingPendingAuthFile struct {
	Provider         string                          `json:"provider"`
	Kind             string                          `json:"kind"`
	CreatedAt        string                          `json:"createdAt"`
	State            string                          `json:"state"`
	Request          enableBankingPendingAuthRequest `json:"request"`
	AuthorizationURL string                          `json:"authorizationUrl"`
	AuthID           string                          `json:"authId,omitempty"`
	Raw              map[string]any                  `json:"raw"`
}

type enableBankingSessionFile struct {
	Provider           string           `json:"provider"`
	CreatedAt          string           `json:"createdAt"`
	Country            string           `json:"country"`
	ASPSPName          string           `json:"aspspName"`
	PSUType            string           `json:"psuType"`
	SessionID          string           `json:"sessionId"`
	AccessValidForDays int              `json:"accessValidForDays,omitempty"`
	Accounts           []map[string]any `json:"accounts,omitempty"`
	Raw                map[string]any   `json:"raw"`
}

type enableBankingStartAuthParams struct {
	Country     string
	ASPSPName   string
	PSUType     string
	ValidDays   int
	RedirectURL string
	AuthFile    string
	OpenBrowser bool
}

type enableBankingFinishSessionParams struct {
	AuthFile    string
	Code        string
	State       string
	SessionFile string
}

type enableBankingConnectParams struct {
	Country            string
	ASPSPName          string
	PSUType            string
	ValidDays          int
	CallbackListenAddr string
	CallbackCertFile   string
	CallbackKeyFile    string
	SessionFile        string
	OpenBrowser        bool
}

func newEnableBankingStartAuthCmd(
	deps financePOCCommandDeps,
	requestParams *enableBankingCommandParams,
) *cobra.Command {
	commandParams := enableBankingStartAuthParams{}
	cmd := &cobra.Command{
		Use:   enableBankingStartAuthOp,
		Short: "Start Enable Banking authorization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := newEnableBankingBaseRequest(cmd, requestParams)
			request.Operation = enableBankingStartAuthOp

			pendingAuth, err := runEnableBankingStartAuth(cmd.Context(), deps, request, commandParams)
			if err != nil {
				return err
			}
			writeErr := writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				commandParams.AuthFile,
				requestParams.JSON,
				pendingAuth,
				fmt.Sprintf("authorization_url: %s\n", pendingAuth.AuthorizationURL),
			)
			if writeErr != nil {
				return writeErr
			}
			writeFinancePOCProgressf(cmd.ErrOrStderr(), "authorization URL: %s", pendingAuth.AuthorizationURL)
			if commandParams.OpenBrowser {
				browserOpener := deps.EnableBankingOpenBrowser
				if browserOpener == nil {
					browserOpener = openFinancePOCBrowser
				}
				openErr := browserOpener(pendingAuth.AuthorizationURL)
				if openErr != nil {
					return fmt.Errorf("open enable-banking browser: %w", openErr)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&commandParams.Country, "country", "", "Two-letter country code")
	cmd.Flags().StringVar(&commandParams.ASPSPName, "aspsp-name", "", "ASPSP name")
	cmd.Flags().StringVar(&commandParams.PSUType, "psu-type", "", "PSU type")
	cmd.Flags().IntVar(&commandParams.ValidDays, "valid-days", 0, "Access validity in days")
	cmd.Flags().StringVar(&commandParams.RedirectURL, "redirect-url", "", "Redirect URL")
	cmd.Flags().StringVar(&commandParams.AuthFile, "auth-file", "", "Pending auth output file")
	cmd.Flags().BoolVar(
		&commandParams.OpenBrowser,
		"open-browser",
		false,
		"Open the authorization URL in a browser",
	)
	_ = cmd.MarkFlagRequired("country")
	_ = cmd.MarkFlagRequired("aspsp-name")
	_ = cmd.MarkFlagRequired("psu-type")
	_ = cmd.MarkFlagRequired("valid-days")
	_ = cmd.MarkFlagRequired("redirect-url")
	_ = cmd.MarkFlagRequired("auth-file")
	return cmd
}

func newEnableBankingFinishSessionCmd(
	deps financePOCCommandDeps,
	requestParams *enableBankingCommandParams,
) *cobra.Command {
	commandParams := enableBankingFinishSessionParams{}
	cmd := &cobra.Command{
		Use:   enableBankingFinishSessionOp,
		Short: "Finish Enable Banking session creation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := newEnableBankingBaseRequest(cmd, requestParams)
			request.Operation = enableBankingFinishSessionOp

			sessionFile, err := runEnableBankingFinishSession(cmd.Context(), deps, request, commandParams)
			if err != nil {
				return err
			}
			return writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				commandParams.SessionFile,
				requestParams.JSON,
				sessionFile,
				fmt.Sprintf("session_id: %s\n", sessionFile.SessionID),
			)
		},
	}
	cmd.Flags().StringVar(&commandParams.AuthFile, "auth-file", "", "Pending auth file")
	cmd.Flags().StringVar(&commandParams.Code, "code", "", "Authorization code")
	cmd.Flags().StringVar(&commandParams.State, "state", "", "Callback state")
	cmd.Flags().StringVar(&commandParams.SessionFile, "session-file", "", "Session output file")
	_ = cmd.MarkFlagRequired("auth-file")
	_ = cmd.MarkFlagRequired("code")
	_ = cmd.MarkFlagRequired("state")
	_ = cmd.MarkFlagRequired("session-file")
	return cmd
}

func newEnableBankingConnectCmd(
	deps financePOCCommandDeps,
	requestParams *enableBankingCommandParams,
) *cobra.Command {
	commandParams := enableBankingConnectParams{}
	cmd := &cobra.Command{
		Use:   enableBankingConnectOp,
		Short: "Start auth and finish via local callback",
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := newEnableBankingBaseRequest(cmd, requestParams)
			request.Operation = enableBankingConnectOp

			sessionFile, err := runEnableBankingConnect(
				cmd.Context(),
				cmd.ErrOrStderr(),
				deps,
				request,
				commandParams,
			)
			if err != nil {
				return err
			}
			return writeFinancePOCJSONPayload(
				cmd.OutOrStdout(),
				commandParams.SessionFile,
				requestParams.JSON,
				sessionFile,
				fmt.Sprintf("session_id: %s\n", sessionFile.SessionID),
			)
		},
	}
	cmd.Flags().StringVar(&commandParams.Country, "country", "", "Two-letter country code")
	cmd.Flags().StringVar(&commandParams.ASPSPName, "aspsp-name", "", "ASPSP name")
	cmd.Flags().StringVar(&commandParams.PSUType, "psu-type", "", "PSU type")
	cmd.Flags().IntVar(&commandParams.ValidDays, "valid-days", 0, "Access validity in days")
	cmd.Flags().StringVar(
		&commandParams.CallbackListenAddr,
		"callback-listen-addr",
		"",
		"Local callback listen address",
	)
	cmd.Flags().StringVar(&commandParams.CallbackCertFile, "callback-cert-file", "", "HTTPS callback certificate file")
	cmd.Flags().StringVar(&commandParams.CallbackKeyFile, "callback-key-file", "", "HTTPS callback private key file")
	cmd.Flags().StringVar(&commandParams.SessionFile, "session-file", "", "Session output file")
	cmd.Flags().BoolVar(
		&commandParams.OpenBrowser,
		"open-browser",
		false,
		"Open the authorization URL in a browser",
	)
	_ = cmd.MarkFlagRequired("country")
	_ = cmd.MarkFlagRequired("aspsp-name")
	_ = cmd.MarkFlagRequired("psu-type")
	_ = cmd.MarkFlagRequired("valid-days")
	_ = cmd.MarkFlagRequired("callback-listen-addr")
	_ = cmd.MarkFlagRequired("session-file")
	return cmd
}

func newEnableBankingBaseRequest(
	cmd *cobra.Command,
	requestParams *enableBankingCommandParams,
) financePOCProviderRequest {
	return financePOCProviderRequest{
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
}

func runEnableBankingStartAuth(
	ctx context.Context,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params enableBankingStartAuthParams,
) (enableBankingPendingAuthFile, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()
	stateGenerator := deps.EnableBankingState
	if stateGenerator == nil {
		stateGenerator = newEnableBankingState
	}
	state, err := stateGenerator()
	if err != nil {
		return enableBankingPendingAuthFile{}, fmt.Errorf("generate enable-banking state: %w", err)
	}

	pendingAuth, err := startEnableBankingAuthorization(ctx, request, enableBankingPendingAuthRequest{
		Country:     strings.TrimSpace(params.Country),
		ASPSPName:   strings.TrimSpace(params.ASPSPName),
		PSUType:     strings.TrimSpace(params.PSUType),
		ValidDays:   params.ValidDays,
		RedirectURL: strings.TrimSpace(params.RedirectURL),
	}, state, deps.Now)
	if err != nil {
		return enableBankingPendingAuthFile{}, err
	}

	return pendingAuth, nil
}

func runEnableBankingFinishSession(
	ctx context.Context,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params enableBankingFinishSessionParams,
) (enableBankingSessionFile, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()
	pendingAuth, err := loadEnableBankingPendingAuthFile(strings.TrimSpace(params.AuthFile))
	if err != nil {
		return enableBankingSessionFile{}, err
	}
	if pendingAuth.State != strings.TrimSpace(params.State) {
		return enableBankingSessionFile{}, errors.New("enable-banking state mismatch")
	}
	return exchangeEnableBankingSession(
		ctx,
		request,
		pendingAuth,
		strings.TrimSpace(params.Code),
		strings.TrimSpace(params.SessionFile),
		deps.Now,
	)
}

func runEnableBankingConnect(
	ctx context.Context,
	stderr io.Writer,
	deps financePOCCommandDeps,
	request financePOCProviderRequest,
	params enableBankingConnectParams,
) (enableBankingSessionFile, error) {
	ctx, cancel := withFinancePOCTimeout(ctx, request.Timeout)
	defer cancel()
	if err := validateEnableBankingConnectCallbackCertFlags(params); err != nil {
		return enableBankingSessionFile{}, err
	}
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "tcp", strings.TrimSpace(params.CallbackListenAddr))
	if err != nil {
		return enableBankingSessionFile{}, fmt.Errorf("listen for enable-banking callback: %w", err)
	}
	defer listener.Close()

	tlsCertificate, usedFallbackCertificate, err := loadEnableBankingCallbackCertificate(listener.Addr(), params)
	if err != nil {
		return enableBankingSessionFile{}, err
	}
	if usedFallbackCertificate {
		writeFinancePOCProgressf(
			stderr,
			"warning: using ephemeral self-signed HTTPS callback certificate; browser may warn unless you provide a trusted local cert via --callback-cert-file and --callback-key-file",
		)
	}

	callbackURL, err := makeEnableBankingCallbackURL(strings.TrimSpace(params.CallbackListenAddr), listener.Addr())
	if err != nil {
		return enableBankingSessionFile{}, err
	}
	pendingAuth, err := runEnableBankingStartAuth(ctx, deps, request, enableBankingStartAuthParams{
		Country:     params.Country,
		ASPSPName:   params.ASPSPName,
		PSUType:     params.PSUType,
		ValidDays:   params.ValidDays,
		RedirectURL: callbackURL,
		OpenBrowser: params.OpenBrowser,
	})
	if err != nil {
		return enableBankingSessionFile{}, err
	}

	writeFinancePOCProgressf(stderr, "authorization URL: %s", pendingAuth.AuthorizationURL)
	if params.OpenBrowser {
		browserOpener := deps.EnableBankingOpenBrowser
		if browserOpener == nil {
			browserOpener = openFinancePOCBrowser
		}
		openErr := browserOpener(pendingAuth.AuthorizationURL)
		if openErr != nil {
			return enableBankingSessionFile{}, fmt.Errorf("open enable-banking browser: %w", openErr)
		}
	}

	writeFinancePOCProgressf(stderr, "waiting for callback on %s", callbackURL)
	callbackListener := tls.NewListener(listener, &tls.Config{
		Certificates: []tls.Certificate{tlsCertificate},
		MinVersion:   tls.VersionTLS12,
	})
	callbackResult, err := waitForEnableBankingCallback(ctx, callbackListener, pendingAuth.State)
	if err != nil {
		return enableBankingSessionFile{}, err
	}

	return exchangeEnableBankingSession(
		ctx,
		request,
		pendingAuth,
		callbackResult.Code,
		strings.TrimSpace(params.SessionFile),
		deps.Now,
	)
}

func validateEnableBankingConnectCallbackCertFlags(params enableBankingConnectParams) error {
	callbackCertFile := strings.TrimSpace(params.CallbackCertFile)
	callbackKeyFile := strings.TrimSpace(params.CallbackKeyFile)
	if callbackCertFile == "" && callbackKeyFile == "" {
		return nil
	}
	if callbackCertFile == "" || callbackKeyFile == "" {
		return errors.New("enable-banking callback cert and key files must be provided together")
	}
	return nil
}

func makeEnableBankingCallbackURL(requestedListenAddr string, listenerAddr net.Addr) (string, error) {
	host, _, err := net.SplitHostPort(requestedListenAddr)
	if err != nil {
		return "", fmt.Errorf("split enable-banking callback listen host: %w", err)
	}
	trimmedHost := strings.TrimSpace(strings.Trim(host, "[]"))
	if trimmedHost == "" {
		return "", errors.New("enable-banking callback listen host is required")
	}

	if tcpAddr, ok := listenerAddr.(*net.TCPAddr); ok {
		return "https://" + net.JoinHostPort(trimmedHost, strconv.Itoa(tcpAddr.Port)) + "/callback", nil
	}

	_, port, err := net.SplitHostPort(listenerAddr.String())
	if err != nil {
		return "", fmt.Errorf("split enable-banking callback listener address: %w", err)
	}

	return "https://" + net.JoinHostPort(trimmedHost, port) + "/callback", nil
}

func loadEnableBankingCallbackCertificate(
	listenerAddr net.Addr,
	params enableBankingConnectParams,
) (tls.Certificate, bool, error) {
	callbackCertFile := strings.TrimSpace(params.CallbackCertFile)
	callbackKeyFile := strings.TrimSpace(params.CallbackKeyFile)
	if callbackCertFile != "" && callbackKeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(callbackCertFile, callbackKeyFile)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("load enable-banking callback TLS certificate: %w", err)
		}
		return certificate, false, nil
	}

	certificate, err := newEnableBankingEphemeralCallbackCertificate(listenerAddr)
	if err != nil {
		return tls.Certificate{}, false, fmt.Errorf("generate enable-banking callback TLS certificate: %w", err)
	}
	return certificate, true, nil
}

func newEnableBankingEphemeralCallbackCertificate(listenerAddr net.Addr) (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate RSA key: %w", err)
	}

	hostNames := map[string]struct{}{enableBankingLocalhost: {}}
	ipAddresses := map[string]net.IP{
		"127.0.0.1": net.ParseIP("127.0.0.1"),
		"::1":       net.ParseIP("::1"),
	}
	if tcpAddr, ok := listenerAddr.(*net.TCPAddr); ok && tcpAddr.IP != nil {
		ipAddresses[tcpAddr.IP.String()] = tcpAddr.IP
	}
	if host, _, splitErr := net.SplitHostPort(listenerAddr.String()); splitErr == nil {
		trimmedHost := strings.TrimSpace(strings.Trim(host, "[]"))
		if parsedIP := net.ParseIP(trimmedHost); parsedIP != nil {
			ipAddresses[parsedIP.String()] = parsedIP
		} else if trimmedHost != "" {
			hostNames[trimmedHost] = struct{}{}
		}
	}

	dnsNames := make([]string, 0, len(hostNames))
	for hostName := range hostNames {
		dnsNames = append(dnsNames, hostName)
	}
	sanIPs := make([]net.IP, 0, len(ipAddresses))
	for _, ipAddress := range ipAddresses {
		sanIPs = append(sanIPs, ipAddress)
	}

	certificateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: enableBankingLocalhost},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames:    dnsNames,
		IPAddresses: sanIPs,
	}

	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		certificateTemplate,
		certificateTemplate,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal private key: %w", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: enableBankingPrivateKeyType, Bytes: privateKeyDER})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated certificate: %w", err)
	}
	return certificate, nil
}

func startEnableBankingAuthorization(
	ctx context.Context,
	request financePOCProviderRequest,
	pendingRequest enableBankingPendingAuthRequest,
	state string,
	now func() time.Time,
) (enableBankingPendingAuthFile, error) {
	if now == nil {
		now = time.Now
	}
	if err := validateEnableBankingStartAuthRequest(request, pendingRequest); err != nil {
		return enableBankingPendingAuthFile{}, err
	}

	raw, err := callEnableBankingJSONEndpoint(
		ctx,
		request,
		http.MethodPost,
		"/auth",
		buildEnableBankingAuthorizationRequestBody(pendingRequest, state, now()),
		now,
	)
	if err != nil {
		return enableBankingPendingAuthFile{}, err
	}

	pendingAuth := enableBankingPendingAuthFile{
		Provider:         enableBankingCommandName,
		Kind:             enableBankingPendingAuthKind,
		CreatedAt:        now().Format(time.RFC3339),
		State:            state,
		Request:          pendingRequest,
		AuthorizationURL: extractEnableBankingString(raw, "url", "authorization_url"),
		AuthID:           extractEnableBankingSessionIdentifier(raw, "authorization_id", "id", "auth_id", "session_id"),
		Raw:              raw,
	}
	if strings.TrimSpace(pendingAuth.AuthorizationURL) == "" {
		return enableBankingPendingAuthFile{}, errors.New("enable-banking auth response missing authorization URL")
	}
	return pendingAuth, nil
}

func exchangeEnableBankingSession(
	ctx context.Context,
	request financePOCProviderRequest,
	pendingAuth enableBankingPendingAuthFile,
	code string,
	sessionFilePath string,
	now func() time.Time,
) (enableBankingSessionFile, error) {
	if now == nil {
		now = time.Now
	}
	if strings.TrimSpace(code) == "" {
		return enableBankingSessionFile{}, errors.New("enable-banking code is required")
	}
	if strings.TrimSpace(sessionFilePath) == "" {
		return enableBankingSessionFile{}, errors.New("enable-banking session file is required")
	}
	if err := validateEnableBankingCredentials(request); err != nil {
		return enableBankingSessionFile{}, err
	}

	raw, err := callEnableBankingJSONEndpoint(ctx, request, http.MethodPost, "/sessions", map[string]any{
		"code": strings.TrimSpace(code),
	}, now)
	if err != nil {
		return enableBankingSessionFile{}, err
	}

	sessionFile := enableBankingSessionFile{
		Provider:           enableBankingCommandName,
		CreatedAt:          now().Format(time.RFC3339),
		Country:            pendingAuth.Request.Country,
		ASPSPName:          pendingAuth.Request.ASPSPName,
		PSUType:            pendingAuth.Request.PSUType,
		SessionID:          extractEnableBankingSessionIdentifier(raw, "id", "session_id"),
		AccessValidForDays: extractEnableBankingNestedInt(raw, "access", "valid_for_days"),
		Accounts:           extractEnableBankingAccounts(raw),
		Raw:                raw,
	}
	if strings.TrimSpace(sessionFile.SessionID) == "" {
		return enableBankingSessionFile{}, errors.New("enable-banking session response missing session ID")
	}
	return sessionFile, nil
}

func buildEnableBankingAuthorizationRequestBody(
	pendingRequest enableBankingPendingAuthRequest,
	state string,
	now time.Time,
) map[string]any {
	validUntil := now.Add(time.Duration(pendingRequest.ValidDays) * 24 * time.Hour)
	return map[string]any{
		"access": map[string]any{
			"valid_until": validUntil.Format(time.RFC3339),
		},
		"aspsp": map[string]any{
			"name":    strings.TrimSpace(pendingRequest.ASPSPName),
			"country": strings.TrimSpace(pendingRequest.Country),
		},
		"state":        strings.TrimSpace(state),
		"redirect_url": strings.TrimSpace(pendingRequest.RedirectURL),
		"psu_type":     strings.TrimSpace(pendingRequest.PSUType),
	}
}

func validateEnableBankingStartAuthRequest(
	request financePOCProviderRequest,
	pendingRequest enableBankingPendingAuthRequest,
) error {
	if err := validateEnableBankingCredentials(request); err != nil {
		return err
	}
	if strings.TrimSpace(pendingRequest.Country) == "" {
		return errors.New("enable-banking country is required")
	}
	if strings.TrimSpace(pendingRequest.ASPSPName) == "" {
		return errors.New("enable-banking aspsp name is required")
	}
	if strings.TrimSpace(pendingRequest.PSUType) == "" {
		return errors.New("enable-banking psu type is required")
	}
	if pendingRequest.ValidDays <= 0 {
		return errors.New("enable-banking valid days must be greater than zero")
	}
	if strings.TrimSpace(pendingRequest.RedirectURL) == "" {
		return errors.New("enable-banking redirect URL is required")
	}
	return nil
}

func validateEnableBankingCredentials(request financePOCProviderRequest) error {
	if strings.TrimSpace(request.AppID) == "" {
		return errors.New("enable-banking app ID is required")
	}
	if strings.TrimSpace(request.PrivateKeyPath) == "" {
		return errors.New("enable-banking private key path is required")
	}
	return nil
}

func loadEnableBankingPendingAuthFile(path string) (enableBankingPendingAuthFile, error) {
	filePayload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return enableBankingPendingAuthFile{}, fmt.Errorf("read enable-banking auth file: %w", err)
	}
	var pendingAuth enableBankingPendingAuthFile
	decodeErr := json.Unmarshal(filePayload, &pendingAuth)
	if decodeErr != nil {
		return enableBankingPendingAuthFile{}, fmt.Errorf("decode enable-banking auth file: %w", decodeErr)
	}
	return pendingAuth, nil
}

func loadEnableBankingSessionFile(path string) (enableBankingSessionFile, error) {
	filePayload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return enableBankingSessionFile{}, fmt.Errorf("read enable-banking session file: %w", err)
	}
	var sessionFile enableBankingSessionFile
	decodeErr := json.Unmarshal(filePayload, &sessionFile)
	if decodeErr != nil {
		return enableBankingSessionFile{}, fmt.Errorf("decode enable-banking session file: %w", decodeErr)
	}
	return sessionFile, nil
}

func callEnableBankingJSONEndpoint(
	ctx context.Context,
	request financePOCProviderRequest,
	method string,
	path string,
	body any,
	now func() time.Time,
) (map[string]any, error) {
	return callEnableBankingJSONEndpointWithQuery(ctx, request, method, path, nil, body, now)
}

func callEnableBankingJSONEndpointWithQuery(
	ctx context.Context,
	request financePOCProviderRequest,
	method string,
	path string,
	query url.Values,
	body any,
	now func() time.Time,
) (map[string]any, error) {
	accessToken, err := newEnableBankingJWT(now(), request.AppID, request.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(strings.TrimRight(request.BaseURL, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("parse enable-banking %s URL: %w", path, err)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var requestBody io.Reader
	if body != nil {
		payload, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal enable-banking %s request: %w", path, marshalErr)
		}
		requestBody = strings.NewReader(string(payload))
	}

	httpRequest, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return nil, fmt.Errorf("build enable-banking %s request: %w", path, err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	httpRequest.Header.Set("Accept", "application/json")
	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("request enable-banking %s: %w", path, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read enable-banking %s response: %w", path, err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, newFinancePOCProviderResponseError(
			enableBankingCommandName,
			strings.TrimPrefix(path, "/"),
			response.StatusCode,
			responseBody,
		)
	}

	var raw map[string]any
	decodeErr := json.Unmarshal(responseBody, &raw)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode enable-banking %s response: %w", path, decodeErr)
	}
	return raw, nil
}

type enableBankingCallbackResult struct {
	Code string
}

func waitForEnableBankingCallback(
	ctx context.Context,
	listener net.Listener,
	expectedState string,
) (enableBankingCallbackResult, error) {
	resultChan := make(chan enableBankingCallbackResult, 1)
	errChan := make(chan error, 1)
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := strings.TrimSpace(r.URL.Query().Get("state"))
			code := strings.TrimSpace(r.URL.Query().Get("code"))
			if state != expectedState {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				select {
				case errChan <- errors.New("enable-banking state mismatch"):
				default:
				}
				return
			}
			if code == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				select {
				case errChan <- errors.New("enable-banking callback code is required"):
				default:
				}
				return
			}
			_, _ = io.WriteString(w, "enable-banking callback received\n")
			select {
			case resultChan <- enableBankingCallbackResult{Code: code}:
			default:
			}
		}),
	}

	serveErrChan := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrChan <- err
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	select {
	case <-ctx.Done():
		return enableBankingCallbackResult{}, fmt.Errorf("wait for enable-banking callback: %w", ctx.Err())
	case err := <-errChan:
		return enableBankingCallbackResult{}, err
	case err := <-serveErrChan:
		return enableBankingCallbackResult{}, fmt.Errorf("serve enable-banking callback: %w", err)
	case result := <-resultChan:
		return result, nil
	}
}

func newEnableBankingState() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func openFinancePOCBrowser(targetURL string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(context.Background(), "open", targetURL)
	case "windows":
		command = exec.CommandContext(
			context.Background(),
			"rundll32",
			"url.dll,FileProtocolHandler",
			targetURL,
		)
	default:
		command = exec.CommandContext(context.Background(), "xdg-open", targetURL)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start browser command: %w", err)
	}
	return nil
}

func extractEnableBankingString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok {
			return value
		}
	}
	return ""
}

func extractEnableBankingSessionIdentifier(raw map[string]any, keys ...string) string {
	identifier := extractEnableBankingString(raw, keys...)
	if identifier != "" {
		return identifier
	}

	parent, ok := raw["session"].(map[string]any)
	if !ok {
		return ""
	}

	return extractEnableBankingString(parent, keys...)
}

func extractEnableBankingNestedInt(raw map[string]any, parentKey string, key string) int {
	parent, ok := raw[parentKey].(map[string]any)
	if !ok {
		return 0
	}
	return extractEnableBankingNumber(parent[key])
}

func extractEnableBankingAccounts(raw map[string]any) []map[string]any {
	items, ok := raw["accounts"].([]any)
	if !ok {
		return nil
	}
	accounts := make([]map[string]any, 0, len(items))
	for _, item := range items {
		account, okCast := item.(map[string]any)
		if okCast {
			accounts = append(accounts, account)
		}
	}
	return accounts
}

func extractEnableBankingNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func withFinancePOCTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
