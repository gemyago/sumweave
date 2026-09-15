// Package swmdclient provides the HTTP-only client used by the swmd command.
package swmdclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	configDirectoryName = "sumweave"
	configFileName      = "swmd.json"
)

// Config contains the only persisted swmd settings.
type Config struct {
	BaseURL  string `json:"baseUrl"`
	APIToken string `json:"apiToken"`
}

// Resolver resolves the local client configuration without app configuration.
type Resolver struct {
	Getenv        func(string) string
	UserConfigDir func() (string, error)
}

// NewResolver creates a resolver backed by the process environment and OS config directory.
func NewResolver() *Resolver {
	return &Resolver{Getenv: os.Getenv, UserConfigDir: os.UserConfigDir}
}

// DefaultConfigPath returns the default swmd configuration file path.
func (r *Resolver) DefaultConfigPath() (string, error) {
	configDir, err := r.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config directory: %w", err)
	}

	return filepath.Join(configDir, configDirectoryName, configFileName), nil
}

// ConfigPath returns selectedPath when present or the default config path otherwise.
func (r *Resolver) ConfigPath(selectedPath string) (string, error) {
	if selectedPath != "" {
		return selectedPath, nil
	}

	return r.DefaultConfigPath()
}

// Resolve applies explicit URL, environment, then file precedence per value.
func (r *Resolver) Resolve(explicitBaseURL, selectedPath string) (Config, string, error) {
	configPath, err := r.ConfigPath(selectedPath)
	if err != nil {
		return Config{}, "", err
	}

	fileConfig, err := readConfig(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, "", err
	}

	baseURL := fileConfig.BaseURL
	if value := r.Getenv("SUMWEAVE_BASE_URL"); value != "" {
		baseURL = value
	}
	if explicitBaseURL != "" {
		baseURL = explicitBaseURL
	}

	apiToken := fileConfig.APIToken
	if value := r.Getenv("SUMWEAVE_API_TOKEN"); value != "" {
		apiToken = value
	}

	return Config{BaseURL: baseURL, APIToken: apiToken}, configPath, nil
}

// WriteConfig atomically replaces configPath with a private configuration file.
//
//nolint:govet // Error scopes are intentionally limited to individual file operations.
func WriteConfig(configPath string, config Config) error {
	if err := validateConfigValues(config); err != nil {
		return err
	}

	directory := filepath.Dir(configPath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}

	encoded := []byte(
		`{"baseUrl":` + strconv.Quote(config.BaseURL) +
			`,"apiToken":` + strconv.Quote(config.APIToken) + `}`,
	)
	encoded = append(encoded, '\n')

	temporary, err := os.CreateTemp(directory, ".swmd-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary configuration mode: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return fmt.Errorf("replace configuration: %w", err)
	}

	return nil
}

// ClearConfig removes persisted configuration. A missing file is already clear.
func ClearConfig(configPath string) error {
	if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove configuration: %w", err)
	}

	return nil
}

// ReadToken reads a single nonempty token value from stdin without echoing it.
func ReadToken(stdin io.Reader) (string, error) {
	value, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read API token from stdin: %w", err)
	}
	token := strings.TrimSpace(string(value))
	if token == "" {
		return "", errors.New("API token on stdin must not be empty")
	}
	return token, nil
}

//nolint:govet // Error scopes are intentionally limited to individual decode operations.
func readConfig(configPath string) (Config, error) {
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Config{}, errors.New("decode configuration: expected one JSON value")
	}

	return config, nil
}

func validateConfigValues(config Config) error {
	if strings.TrimSpace(config.BaseURL) == "" {
		return errors.New("base URL must not be empty")
	}
	if strings.TrimSpace(config.APIToken) == "" {
		return errors.New("API token must not be empty")
	}
	return nil
}
