package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancePOCCmd(t *testing.T) {
	fake := faker.New()

	type commandResult struct {
		Provider  string         `json:"provider"`
		Operation string         `json:"operation"`
		FetchedAt string         `json:"fetchedAt"`
		Summary   map[string]any `json:"summary"`
		Raw       map[string]any `json:"raw,omitempty"`
	}

	makeRootCmd := func(t *testing.T, deps financePOCCommandDeps) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.AddCommand(newFinancePOCCmd(deps))
		return rootCmd, stdout, stderr
	}

	t.Run("setupCommands", func(t *testing.T) {
		t.Run("wires finance-poc with provider roots", func(t *testing.T) {
			rootCmd := setupCommands()
			financePOCCmd := findCommandByName(t, rootCmd, financePOCCommandName)
			assert.Equal(t, financePOCCommandName, financePOCCmd.Name())
			assert.NotNil(t, findCommandByName(t, financePOCCmd, enableBankingCommandName))
			assert.NotNil(t, findCommandByName(t, financePOCCmd, monobankCommandName))
		})
	})

	t.Run("provider skeleton execution", func(t *testing.T) {
		t.Run("--json writes machine-readable stdout and progress to stderr", func(t *testing.T) {
			fetchedAt := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
			rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{
				Now: func() time.Time { return fetchedAt },
				EnableBankingRunner: func(_ context.Context, request financePOCProviderRequest) (financePOCProviderResult, error) {
					return financePOCProviderResult{
						Summary: map[string]any{
							"baseURL": request.BaseURL,
							"timeout": request.Timeout.String(),
						},
						Raw: map[string]any{"provider": request.Provider},
					}, nil
				},
			})

			rootCmd.SetArgs([]string{"finance-poc", "enable-banking", "--json"})
			require.NoError(t, rootCmd.ExecuteContext(t.Context()))

			var got commandResult
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
			assert.Equal(t, enableBankingCommandName, got.Provider)
			assert.Equal(t, financePOCOperationStatus, got.Operation)
			assert.Equal(t, fetchedAt.Format(time.RFC3339), got.FetchedAt)
			assert.Equal(t, enableBankingDefaultBaseURL, got.Summary["baseURL"])
			assert.Contains(t, stderr.String(), "resolved finance-poc provider configuration")
			assert.NotContains(t, stdout.String(), "resolved finance-poc provider configuration")
		})

		t.Run("flags override env without leaking token values", func(t *testing.T) {
			envToken := fake.Internet().Password() + "-env"
			flagToken := fake.Internet().Password() + "-flag"
			envBaseURL := fmt.Sprintf("https://%s.example.test", fake.Lorem().Word())
			flagBaseURL := fmt.Sprintf("https://%s.example.test", fake.Lorem().Word())
			t.Setenv("MONOBANK_TOKEN", envToken)
			t.Setenv("MONOBANK_BASE_URL", envBaseURL)

			var captured financePOCProviderRequest
			rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{
				MonobankRunner: func(_ context.Context, request financePOCProviderRequest) (financePOCProviderResult, error) {
					captured = request
					return financePOCProviderResult{
						Summary: map[string]any{
							"baseURL":         request.BaseURL,
							"tokenConfigured": request.Token != "",
							"tokenSource":     request.TokenSource,
						},
					}, nil
				},
			})

			rootCmd.SetArgs([]string{
				"finance-poc", "monobank", "--json",
				"--base-url", flagBaseURL,
				"--token", flagToken,
			})
			require.NoError(t, rootCmd.ExecuteContext(t.Context()))
			assert.Equal(t, flagBaseURL, captured.BaseURL)
			assert.Equal(t, flagToken, captured.Token)
			assert.Equal(t, financePOCTokenSourceFlag, captured.TokenSource)
			assert.NotContains(t, stdout.String(), envToken)
			assert.NotContains(t, stdout.String(), flagToken)
			assert.NotContains(t, stderr.String(), envToken)
			assert.NotContains(t, stderr.String(), flagToken)
		})

		t.Run("--out writes owner-only file and creates parents", func(t *testing.T) {
			outFile := filepath.Join(t.TempDir(), fake.Lorem().Word(), fake.Lorem().Word()+".json")
			rootCmd, stdout, _ := makeRootCmd(t, financePOCCommandDeps{
				EnableBankingRunner: func(_ context.Context, request financePOCProviderRequest) (financePOCProviderResult, error) {
					return financePOCProviderResult{Summary: map[string]any{"baseURL": request.BaseURL}}, nil
				},
			})

			rootCmd.SetArgs([]string{"finance-poc", "enable-banking", "--json", "--out", outFile})
			require.NoError(t, rootCmd.ExecuteContext(t.Context()))

			written, err := os.ReadFile(outFile)
			require.NoError(t, err)
			assert.JSONEq(t, stdout.String(), string(written))

			info, err := os.Stat(outFile)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})

		t.Run("timeout returns sanitized error", func(t *testing.T) {
			secretToken := fake.Internet().Password() + "-token"
			rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{
				MonobankRunner: func(ctx context.Context, _ financePOCProviderRequest) (financePOCProviderResult, error) {
					<-ctx.Done()
					return financePOCProviderResult{}, fmt.Errorf(
						"Authorization: Bearer %s: %w",
						secretToken,
						ctx.Err(),
					)
				},
			})

			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{"finance-poc", "monobank", "--json", "--timeout", "1ms", "--token", secretToken})
			err := rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "timed out")
			assert.NotContains(t, err.Error(), secretToken)
		})
	})

	t.Run("shared helpers", func(t *testing.T) {
		t.Run("provider error excerpt is bounded and redacted", func(t *testing.T) {
			secretToken := fake.Internet().Password() + "-token"
			secretValue := "secret-" + fake.Internet().Password()
			jwt := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.signature"
			privateKey := "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----"
			body := []byte(
				fmt.Sprintf(
					`{"token":"%s","secret":"%s","error":"%s"}%s%s%s`,
					secretToken,
					secretValue,
					fake.Lorem().Sentence(4),
					jwt,
					privateKey,
					string(bytes.Repeat([]byte("a"), financePOCProviderErrorExcerptLimit)),
				),
			)

			err := newFinancePOCProviderResponseError(monobankCommandName, "accounts", 502, body)
			require.Error(t, err)
			require.ErrorContains(t, err, "status 502")
			assert.NotContains(t, err.Error(), secretToken)
			assert.NotContains(t, err.Error(), secretValue)
			assert.NotContains(t, err.Error(), jwt)
			assert.NotContains(t, err.Error(), privateKey)
			assert.Contains(t, err.Error(), `"token":"[REDACTED]"`)
			assert.Contains(t, err.Error(), `"secret":"[REDACTED]"`)
			assert.LessOrEqual(t, len(err.Error()), 512)
		})

		t.Run("parse date validates YYYY-MM-DD", func(t *testing.T) {
			parsed, err := parseFinancePOCDate("--from", "2026-06-18")
			require.NoError(t, err)
			assert.Equal(t, time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC), parsed)

			inclusiveParsed, err := parseFinancePOCInclusiveEndDate("--to", "2026-06-18")
			require.NoError(t, err)
			assert.Equal(t, time.Date(2026, time.June, 18, 23, 59, 59, 0, time.UTC), inclusiveParsed)

			_, err = parseFinancePOCInclusiveEndDate("--to", fake.Lorem().Word())
			require.Error(t, err)
			require.ErrorContains(t, err, "parse --to")

			_, err = parseFinancePOCDate("--from", fake.Lorem().Word())
			require.Error(t, err)
			require.ErrorContains(t, err, "parse --from")
		})

		t.Run("output writing covers text, marshal, and filesystem errors", func(t *testing.T) {
			t.Run("writes text output when json flag is disabled", func(t *testing.T) {
				stdout := &bytes.Buffer{}
				err := writeFinancePOCEnvelope(stdout, "", false, financePOCEnvelope{
					Provider:  monobankCommandName,
					Operation: financePOCOperationStatus,
					FetchedAt: "2026-06-18T12:00:00Z",
					Summary:   map[string]any{"ok": true},
				})
				require.NoError(t, err)
				assert.Contains(t, stdout.String(), "provider: monobank")
			})

			t.Run("returns marshal error for unsupported json values", func(t *testing.T) {
				stdout := &bytes.Buffer{}
				err := writeFinancePOCEnvelope(stdout, "", true, financePOCEnvelope{
					Provider: monobankCommandName,
					Summary:  map[string]any{"bad": func() {}},
				})
				require.Error(t, err)
				require.ErrorContains(t, err, "marshal finance-poc output")
			})

			t.Run("returns filesystem error when parent path is not a directory", func(t *testing.T) {
				blockedParent := filepath.Join(t.TempDir(), fake.Lorem().Word())
				require.NoError(t, os.WriteFile(blockedParent, []byte("x"), 0o600))

				stdout := &bytes.Buffer{}
				err := writeFinancePOCEnvelope(
					stdout,
					filepath.Join(blockedParent, "out.json"),
					true,
					financePOCEnvelope{Provider: monobankCommandName, Summary: map[string]any{"ok": true}},
				)
				require.Error(t, err)
				require.ErrorContains(t, err, "create finance-poc output directory")
			})

			t.Run("progress writer is safe for nil and writes newline", func(t *testing.T) {
				writeFinancePOCProgressf(nil, "ignored %s", fake.Lorem().Word())

				stderr := &bytes.Buffer{}
				writeFinancePOCProgressf(stderr, "progress %s", monobankCommandName)
				assert.Equal(t, "progress monobank\n", stderr.String())
			})
		})
	})
}

func findCommandByName(t *testing.T, parent *cobra.Command, commandName string) *cobra.Command {
	t.Helper()
	for _, cmd := range parent.Commands() {
		if cmd.Name() == commandName {
			return cmd
		}
	}
	t.Fatalf("command %q not found", commandName)
	return nil
}
