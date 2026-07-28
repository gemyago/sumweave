package finance

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type MonobankConfig struct {
	BaseURL    string
	HTTPClient *http.Client
}

type EnableBankingASPSP struct {
	ProviderID domain.ProviderID
	Name       string
	Country    string
	PSUType    string
	ValidDays  int
}

type EnableBankingConfig struct {
	BaseURL        string
	AppID          string
	PrivateKeyPath string
	ASPSPs         []EnableBankingASPSP
}

type Config struct {
	Database               *persistence.Database
	Logger                 *slog.Logger
	Now                    func() time.Time
	NewID                  func() string
	HTTPClient             *http.Client
	ConnectionSecretCipher connectionSecretCipher
	FXProviders            []FXRatesProvider
	DefaultFXProvider      string
	FXJobEnqueuer          FXRefreshJobEnqueuer
	FXScheduleWriter       FXRefreshScheduleWriter
	CSVImportJobEnqueuer   CSVImportJobEnqueuer
	BankSyncJobEnqueuer    BankConnectionSyncJobEnqueuer
	BankSyncScheduleWriter BankConnectionSyncScheduleWriter
	Monobank               MonobankConfig
	EnableBanking          EnableBankingConfig
}

func (c *Config) validate() error {
	if c == nil {
		return errors.New("config is required")
	}

	if c.Database == nil {
		return errors.New("database is required")
	}

	if c.Logger == nil {
		return errors.New("logger is required")
	}

	if c.Now == nil {
		return errors.New("now is required")
	}

	if c.NewID == nil {
		return errors.New("new id is required")
	}

	if c.HTTPClient == nil {
		return errors.New("http client is required")
	}

	if c.ConnectionSecretCipher == nil {
		return errors.New("connection secret cipher is required")
	}

	if err := c.Monobank.validate(); err != nil {
		return fmt.Errorf("validate monobank config: %w", err)
	}

	if err := c.EnableBanking.validate(); err != nil {
		return fmt.Errorf("validate enable banking config: %w", err)
	}

	return nil
}

func (c MonobankConfig) validate() error {
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}
	return nil
}

func (c EnableBankingConfig) validate() error {
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}

	if c.AppID == "" {
		return errors.New("app ID is required")
	}

	if c.PrivateKeyPath == "" {
		return errors.New("private key path is required")
	}

	if len(c.ASPSPs) == 0 {
		return errors.New("at least one ASPSP is required")
	}

	for index, aspsp := range c.ASPSPs {
		if err := aspsp.validate(); err != nil {
			return fmt.Errorf("ASPSP %d: %w", index, err)
		}
	}

	return nil
}

func (c EnableBankingASPSP) validate() error {
	if c.ProviderID == "" {
		return errors.New("provider ID is required")
	}

	if c.Name == "" {
		return errors.New("name is required")
	}

	if c.Country == "" {
		return errors.New("country is required")
	}

	if c.PSUType == "" {
		return errors.New("PSU type is required")
	}

	if c.ValidDays <= 0 {
		return errors.New("valid days must be positive")
	}

	return nil
}
