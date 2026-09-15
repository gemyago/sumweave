package swmdclient

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	t.Run("uses process defaults", func(t *testing.T) {
		resolver := NewResolver()
		require.NotNil(t, resolver.Getenv)
		require.NotNil(t, resolver.UserConfigDir)
	})
	makeResolver := func(environment map[string]string, configDirectory string) *Resolver {
		return &Resolver{
			Getenv:        func(key string) string { return environment[key] },
			UserConfigDir: func() (string, error) { return configDirectory, nil },
		}
	}

	t.Run("resolves precedence and selected paths", func(t *testing.T) {
		fake := faker.New()
		directory := t.TempDir()
		selectedPath := filepath.Join(directory, fake.UUID().V4()+".json")
		fileConfig := Config{BaseURL: "https://file.example.test", APIToken: "file-" + fake.UUID().V4()}
		require.NoError(t, WriteConfig(selectedPath, fileConfig))
		resolver := makeResolver(map[string]string{
			"SUMWEAVE_BASE_URL":  "https://environment.example.test",
			"SUMWEAVE_API_TOKEN": "environment-" + fake.UUID().V4(),
		}, directory)

		config, path, err := resolver.Resolve("https://explicit.example.test", selectedPath)

		require.NoError(t, err)
		require.Equal(t, selectedPath, path)
		require.Equal(t, Config{
			BaseURL:  "https://explicit.example.test",
			APIToken: resolver.Getenv("SUMWEAVE_API_TOKEN"),
		}, config)
	})

	t.Run("uses file values and default path", func(t *testing.T) {
		fake := faker.New()
		directory := t.TempDir()
		resolver := makeResolver(nil, directory)
		defaultPath, err := resolver.DefaultConfigPath()
		require.NoError(t, err)
		want := Config{BaseURL: "https://" + fake.Internet().Domain(), APIToken: fake.UUID().V4()}
		require.NoError(t, WriteConfig(defaultPath, want))

		got, path, err := resolver.Resolve("", "")

		require.NoError(t, err)
		require.Equal(t, defaultPath, path)
		require.Equal(t, want, got)
	})

	t.Run("rejects malformed configuration", func(t *testing.T) {
		directory := t.TempDir()
		configPath := filepath.Join(directory, "malformed.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{"baseUrl":"https://example.test","unknown":true}`), 0o600))

		_, _, err := makeResolver(nil, directory).Resolve("", configPath)

		require.Error(t, err)
	})

	t.Run("reports config directory errors", func(t *testing.T) {
		resolver := &Resolver{
			Getenv:        func(string) string { return "" },
			UserConfigDir: func() (string, error) { return "", errors.New("unavailable") },
		}

		_, err := resolver.DefaultConfigPath()

		require.Error(t, err)
		_, _, err = resolver.Resolve("", "")
		require.Error(t, err)
	})
}

func TestConfigurationPersistence(t *testing.T) {
	t.Run("writes private parent and atomic private file then clears it", func(t *testing.T) {
		fake := faker.New()
		path := filepath.Join(t.TempDir(), "private", fake.UUID().V4(), "swmd.json")
		config := Config{BaseURL: "https://" + fake.Internet().Domain(), APIToken: fake.UUID().V4()}

		require.NoError(t, WriteConfig(path, config))
		fileInfo, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
		parentInfo, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), parentInfo.Mode().Perm())
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		require.JSONEq(t, `{"baseUrl":"`+config.BaseURL+`","apiToken":"`+config.APIToken+`"}`, string(contents))

		require.NoError(t, ClearConfig(path))
		require.NoFileExists(t, path)
		require.NoError(t, ClearConfig(path))
	})

	t.Run("rejects missing persisted values", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "swmd.json")
		for _, config := range []Config{{}, {BaseURL: "https://example.test"}, {APIToken: "token"}} {
			require.Error(t, WriteConfig(path, config))
			require.NoFileExists(t, path)
		}
	})

	t.Run("reports inaccessible persistence and clearing paths", func(t *testing.T) {
		fake := faker.New()
		parentFile := filepath.Join(t.TempDir(), fake.UUID().V4())
		require.NoError(t, os.WriteFile(parentFile, []byte("file"), 0o600))
		require.Error(t, WriteConfig(
			filepath.Join(parentFile, "swmd.json"),
			Config{BaseURL: "https://example.test", APIToken: "token"},
		))

		directory := filepath.Join(t.TempDir(), fake.UUID().V4())
		require.NoError(t, os.Mkdir(directory, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(directory, "child"), []byte("child"), 0o600))
		require.Error(t, ClearConfig(directory))
		_, err := readConfig(filepath.Join(directory, "missing"))
		require.Error(t, err)
	})

	t.Run("reads only nonempty stdin tokens", func(t *testing.T) {
		fake := faker.New()
		token, err := ReadToken(strings.NewReader("\n" + fake.UUID().V4() + "\n"))
		require.NoError(t, err)
		require.NotEmpty(t, token)
		_, err = ReadToken(strings.NewReader(" \n"))
		require.Error(t, err)
		file, fileErr := os.CreateTemp(t.TempDir(), "closed-reader")
		require.NoError(t, fileErr)
		require.NoError(t, file.Close())
		_, err = ReadToken(file)
		require.Error(t, err)
	})

	t.Run("rejects empty and multiple JSON values", func(t *testing.T) {
		directory := t.TempDir()
		for _, contents := range []string{"", `{"baseUrl":"https://example.test","apiToken":"token"} {}`} {
			path := filepath.Join(directory, faker.New().UUID().V4()+".json")
			require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
			_, err := readConfig(path)
			require.Error(t, err)
		}
	})
}
