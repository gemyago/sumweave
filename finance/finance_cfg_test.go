package finance

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	makeConfig := func(t *testing.T) *Config {
		t.Helper()

		fake := faker.New()
		key := sha256.Sum256([]byte("finance-config-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], "finance-config")
		require.NoError(t, err)

		return &Config{
			Database:               openTestDatabase(t),
			Logger:                 slog.New(slog.DiscardHandler),
			Now:                    func() time.Time { return time.Now().UTC() },
			NewID:                  uuid.NewString,
			HTTPClient:             &http.Client{},
			ConnectionSecretCipher: cipher,
			Monobank: MonobankConfig{
				BaseURL: "https://" + fake.Internet().Domain(),
			},
			EnableBanking: EnableBankingConfig{
				BaseURL:        "https://" + fake.Internet().Domain(),
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: filepath.Join(t.TempDir(), "enable-banking-private-key.pem"),
				ASPSPs: []EnableBankingASPSP{{
					ProviderID: domain.ProviderID("provider-" + fake.UUID().V4()),
					Name:       "aspsp-" + fake.Company().Name(),
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  fake.IntBetween(1, 120),
				}},
			},
		}
	}

	t.Run("accepts complete config", func(t *testing.T) {
		require.NoError(t, makeConfig(t).validate())
	})

	t.Run("rejects missing root dependencies", func(t *testing.T) {
		testCases := []struct {
			name        string
			mutate      func(*Config)
			errContains string
		}{
			{
				name:        "nil config",
				mutate:      nil,
				errContains: "config is required",
			},
			{
				name:        "database",
				mutate:      func(cfg *Config) { cfg.Database = nil },
				errContains: "database is required",
			},
			{
				name:        "logger",
				mutate:      func(cfg *Config) { cfg.Logger = nil },
				errContains: "logger is required",
			},
			{
				name:        "now",
				mutate:      func(cfg *Config) { cfg.Now = nil },
				errContains: "now is required",
			},
			{
				name:        "new id",
				mutate:      func(cfg *Config) { cfg.NewID = nil },
				errContains: "new id is required",
			},
			{
				name:        "http client",
				mutate:      func(cfg *Config) { cfg.HTTPClient = nil },
				errContains: "http client is required",
			},
			{
				name:        "cipher",
				mutate:      func(cfg *Config) { cfg.ConnectionSecretCipher = nil },
				errContains: "connection secret cipher is required",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				var cfg *Config
				if testCase.mutate != nil {
					cfg = makeConfig(t)
					testCase.mutate(cfg)
				}

				err := cfg.validate()
				require.ErrorContains(t, err, testCase.errContains)
			})
		}
	})

	t.Run("rejects provider config gaps", func(t *testing.T) {
		testCases := []struct {
			name        string
			mutate      func(*Config)
			errContains string
		}{
			{
				name:        "monobank URL",
				mutate:      func(cfg *Config) { cfg.Monobank.BaseURL = "" },
				errContains: "validate monobank config: base URL is required",
			},
			{
				name:        "enable banking config",
				mutate:      func(cfg *Config) { cfg.EnableBanking = EnableBankingConfig{} },
				errContains: "validate enable banking config: base URL is required",
			},
			{
				name:        "enable banking URL",
				mutate:      func(cfg *Config) { cfg.EnableBanking.BaseURL = "" },
				errContains: "validate enable banking config: base URL is required",
			},
			{
				name: "enable banking app ID",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.AppID = ""
				},
				errContains: "app ID is required",
			},
			{
				name: "enable banking private key",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.PrivateKeyPath = ""
				},
				errContains: "private key path is required",
			},
			{
				name:        "ASPSPs",
				mutate:      func(cfg *Config) { cfg.EnableBanking.ASPSPs = nil },
				errContains: "at least one ASPSP is required",
			},
			{
				name: "ASPSP provider",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.ASPSPs[0].ProviderID = ""
				},
				errContains: "ASPSP 0: provider ID is required",
			},
			{
				name: "ASPSP name",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.ASPSPs[0].Name = ""
				},
				errContains: "ASPSP 0: name is required",
			},
			{
				name: "ASPSP country",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.ASPSPs[0].Country = ""
				},
				errContains: "ASPSP 0: country is required",
			},
			{
				name: "ASPSP PSU type",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.ASPSPs[0].PSUType = ""
				},
				errContains: "ASPSP 0: PSU type is required",
			},
			{
				name: "ASPSP valid days",
				mutate: func(cfg *Config) {
					cfg.EnableBanking.ASPSPs[0].ValidDays = 0
				},
				errContains: "ASPSP 0: valid days must be positive",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				cfg := makeConfig(t)
				testCase.mutate(cfg)

				err := cfg.validate()
				require.ErrorContains(t, err, testCase.errContains)
			})
		}
	})
}
