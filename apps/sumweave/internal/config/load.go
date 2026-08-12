package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

//go:embed *.yaml
var resources embed.FS

func mergeResourceCfg(cfg *viper.Viper, resourceName string) error {
	resourceStream, err := resources.Open(resourceName)
	if err != nil {
		return fmt.Errorf("failed to read config %v: %w", resourceName, err)
	}
	defer resourceStream.Close()

	if err = cfg.MergeConfig(resourceStream); err != nil {
		return fmt.Errorf("failed to load config %v: %w", resourceName, err)
	}
	return nil
}

type LoadOpts struct {
	env                   string
	defaultConfigFileName string
}

// NewLoadOpts creates a new LoadOpts instance.
func NewLoadOpts() *LoadOpts {
	return &LoadOpts{
		env:                   "local",
		defaultConfigFileName: "default.yaml",
	}
}

func (opts *LoadOpts) WithEnv(val string) *LoadOpts {
	if val != "" {
		opts.env = val
	}
	return opts
}

func New() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix("APP")
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(
		strings.NewReplacer("-", "_", ".", "_"),
	)
	v.AutomaticEnv()
	return v
}

func load(cfg *viper.Viper, opts *LoadOpts) error {
	if err := mergeResourceCfg(cfg, opts.defaultConfigFileName); err != nil {
		return err
	}

	if err := mergeResourceCfg(cfg, opts.env+".yaml"); err != nil {
		return err
	}

	// load env user if exists
	if err := mergeResourceCfg(cfg, opts.env+"-user.yaml"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return nil
}
