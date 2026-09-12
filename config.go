package gopinion

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type policyMode string

const (
	policyRequired policyMode = "required"
	policyDisabled policyMode = "disabled"
)

type config struct {
	Version        *int                 `yaml:"version"`
	Server         serverConfig         `yaml:"server"`
	Authentication authenticationConfig `yaml:"authentication"`
	Authorization  authorizationConfig  `yaml:"authorization"`
	Pagination     paginationConfig     `yaml:"pagination"`
}

type serverConfig struct {
	Address           string `yaml:"address"`
	ReadHeaderTimeout string `yaml:"read_header_timeout"`
	ReadTimeout       string `yaml:"read_timeout"`
	WriteTimeout      string `yaml:"write_timeout"`
	IdleTimeout       string `yaml:"idle_timeout"`
	ShutdownTimeout   string `yaml:"shutdown_timeout"`
	MaxBodyBytes      int64  `yaml:"max_body_bytes"`

	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	shutdownTimeout   time.Duration
}

type authenticationConfig struct {
	Mode policyMode `yaml:"mode"`
}

type authorizationConfig struct {
	Mode policyMode `yaml:"mode"`
}

type paginationConfig struct {
	Mode          policyMode `yaml:"mode"`
	DefaultLimit  int        `yaml:"default_limit"`
	MaximumLimit  int        `yaml:"maximum_limit"`
	MaximumOffset int        `yaml:"maximum_offset"`
}

func defaultConfig() config {
	return config{
		Server: serverConfig{
			Address:           ":8080",
			ReadHeaderTimeout: "5s",
			ReadTimeout:       "15s",
			WriteTimeout:      "30s",
			IdleTimeout:       "60s",
			ShutdownTimeout:   "10s",
			MaxBodyBytes:      1 << 20,
		},
		Authentication: authenticationConfig{Mode: policyRequired},
		Authorization:  authorizationConfig{Mode: policyRequired},
		Pagination: paginationConfig{
			Mode:          policyRequired,
			DefaultLimit:  25,
			MaximumLimit:  100,
			MaximumOffset: 10000,
		},
	}
}

func loadConfig(path string) (config, error) {
	file, err := os.Open(path)
	if err != nil {
		return config{}, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()

	cfg := defaultConfig()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("decode configuration: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return config{}, errors.New("decode configuration: multiple YAML documents are not allowed")
		}
		return config{}, fmt.Errorf("decode configuration: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return config{}, fmt.Errorf("validate configuration: %w", err)
	}
	return cfg, nil
}

func (cfg *config) validate() error {
	if cfg.Version == nil {
		return errors.New("version is required")
	}
	if *cfg.Version != 3 {
		return fmt.Errorf("version must be 3, got %d", *cfg.Version)
	}
	if cfg.Server.Address == "" {
		return errors.New("server.address must not be empty")
	}
	if cfg.Server.MaxBodyBytes <= 0 {
		return errors.New("server.max_body_bytes must be greater than zero")
	}

	readHeaderTimeout, err := parsePositiveDuration("server.read_header_timeout", cfg.Server.ReadHeaderTimeout)
	if err != nil {
		return err
	}
	readTimeout, err := parsePositiveDuration("server.read_timeout", cfg.Server.ReadTimeout)
	if err != nil {
		return err
	}
	writeTimeout, err := parsePositiveDuration("server.write_timeout", cfg.Server.WriteTimeout)
	if err != nil {
		return err
	}
	idleTimeout, err := parsePositiveDuration("server.idle_timeout", cfg.Server.IdleTimeout)
	if err != nil {
		return err
	}
	shutdownTimeout, err := parsePositiveDuration("server.shutdown_timeout", cfg.Server.ShutdownTimeout)
	if err != nil {
		return err
	}
	cfg.Server.readHeaderTimeout = readHeaderTimeout
	cfg.Server.readTimeout = readTimeout
	cfg.Server.writeTimeout = writeTimeout
	cfg.Server.idleTimeout = idleTimeout
	cfg.Server.shutdownTimeout = shutdownTimeout

	if err := validatePolicyMode("authentication.mode", cfg.Authentication.Mode); err != nil {
		return err
	}
	if err := validatePolicyMode("authorization.mode", cfg.Authorization.Mode); err != nil {
		return err
	}
	if cfg.Authorization.Mode == policyRequired && cfg.Authentication.Mode == policyDisabled {
		return errors.New("authorization.mode cannot be required when authentication.mode is disabled")
	}
	if err := validatePolicyMode("pagination.mode", cfg.Pagination.Mode); err != nil {
		return err
	}
	if cfg.Pagination.DefaultLimit <= 0 {
		return errors.New("pagination.default_limit must be greater than zero")
	}
	if cfg.Pagination.MaximumLimit < cfg.Pagination.DefaultLimit {
		return errors.New("pagination.maximum_limit must be greater than or equal to pagination.default_limit")
	}
	if cfg.Pagination.MaximumOffset < 0 {
		return errors.New("pagination.maximum_offset must not be negative")
	}
	maximumInteger := int(^uint(0) >> 1)
	if cfg.Pagination.MaximumOffset > maximumInteger-cfg.Pagination.MaximumLimit {
		return errors.New("pagination.maximum_offset plus pagination.maximum_limit is too large")
	}
	return nil
}

func validatePolicyMode(name string, mode policyMode) error {
	switch mode {
	case policyRequired, policyDisabled:
		return nil
	default:
		return fmt.Errorf("%s must be %q or %q, got %q", name, policyRequired, policyDisabled, mode)
	}
}

func parsePositiveDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return duration, nil
}
