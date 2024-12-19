package yamlConfig

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ivanehh/boiler"
	"github.com/ivanehh/boiler/internal/helpers"
	"gopkg.in/yaml.v3"
)

const (
	MySQL   = "mysql"
	MSSQL   = "mssql"
	MongoDB = "mongodb"
	Posgres = "postgres"
	REST    = "rest"
	FTP     = "ftp"
)

type ConfigManager struct {
	current *BaseConfig
	mu      sync.RWMutex
}

var manager = &ConfigManager{}

// BaseConfig implements boiler.Config
type BaseConfig struct {
	Svc  service   `yaml:"service"`
	Srcs []source  `yaml:"sources"`
	Log  logConfig `yaml:"logging,omitempty"`
	Ext  any       `yaml:"extension,omitempty"`
}

func (bc *BaseConfig) Validate() error {
	if bc.Svc.Name == "" {
		return errors.New("service configuration is required")
	}
	if bc.Svc.Port == 0 {
		bc.Svc.Port = 8080 // Default port
	}
	for _, s := range bc.Srcs {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("source %s: %w", s.Nam, err)
		}
	}
	return nil
}

// Implement boiler.Config interface
func (bc *BaseConfig) Sources() []boiler.IOWithAuth {
	result := make([]boiler.IOWithAuth, len(bc.Srcs))
	for i, src := range bc.Srcs {
		result[i] = src
	}
	return result
}

// Getter methods for BaseConfig
func (bc *BaseConfig) Service() service     { return bc.Svc }
func (bc *BaseConfig) LogConfig() logConfig { return bc.Log }
func (bc *BaseConfig) Extension() any       { return bc.Ext }

type service struct {
	Name    string `yaml:"name" env:"SERVICE_NAME"`
	Purpose string `yaml:"purpose" env:"SERVICE_PURPOSE"`
	Port    int    `yaml:"port" env:"SERVICE_PORT" default:"8080"`
}

type source struct {
	Nam   string       `yaml:"name"`
	Typ   string       `yaml:"type"`
	Enbl  bool         `yaml:"enabled"`
	Loc   string       `yaml:"location"`
	Creds *credentials `yaml:"auth,omitempty"`
}

func (s *source) Validate() error {
	if s.Nam == "" {
		return errors.New("name is required")
	}
	if s.Loc == "" {
		return errors.New("location is required")
	}
	if s.Creds != nil {
		if err := s.Creds.Validate(); err != nil {
			return fmt.Errorf("credentials error: %w", err)
		}
	}
	return nil
}

// Implement boiler.IOWithAuth interface
func (s source) Auth() boiler.Credentials {
	if s.Creds == nil {
		return credentials{} // Return empty credentials if none configured
	}
	return *s.Creds
}
func (s source) Enabled() bool { return s.Enbl }
func (s source) Type() string  { return s.Typ }
func (s source) Name() string  { return s.Nam }
func (s source) Addr() string  { return s.Loc }

type credentials struct {
	uname string `yaml:"username"`
	pwd   string `yaml:"password"`
}

func (c *credentials) Validate() error {
	if c.uname == "" {
		return errors.New("username is required when auth is specified")
	}
	if c.pwd == "" {
		return errors.New("password is required when auth is specified")
	}
	return nil
}

func (c credentials) Username() string { return c.uname }
func (c credentials) Password() string { return c.pwd }

type logConfig struct {
	level   string `yaml:"level" env:"LOG_LEVEL" default:"info"`
	folder  string `yaml:"filePath" env:"LOG_PATH"`
	maxSize int    `yaml:"maxSize" env:"LOG_MAX_SIZE" default:"100"`
}

func (lc logConfig) MinLevel() slog.Level {
	switch strings.ToLower(lc.level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (lc logConfig) Dir() string      { return lc.folder }
func (lc logConfig) MaxFileSize() int { return lc.maxSize }

func ExtensionAs[T any](c *BaseConfig) (T, error) {
	var result T
	if c.Ext == nil {
		return result, errors.New("no extension configuration found")
	}

	yamlData, err := yaml.Marshal(c.Ext)
	if err != nil {
		return result, fmt.Errorf("failed to marshal extension: %w", err)
	}

	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal extension to target type: %w", err)
	}

	return result, nil
}

func Load(override string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	config := &BaseConfig{}

	fp := override
	if len(override) == 0 {
		fp = filepath.Join(helpers.Rootpath(), "config", "cfg.yaml")
	}
	if !filepath.IsAbs(fp) {
		fp = "/" + fp
	}

	yamlFile, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(yamlFile, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	manager.current = config
	return nil
}

func Reload(override string) error {
	return Load(override)
}

func Get() *BaseConfig {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if manager.current == nil {
		panic("configuration must be loaded before use")
	}
	return manager.current
}
