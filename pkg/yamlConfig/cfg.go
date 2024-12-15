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
	svc     *service  `yaml:"service"`
	sources []source  `yaml:"sources"`
	log     logConfig `yaml:"logging"`
	ext     any       `yaml:"extension,omitempty"`
}

func (bc *BaseConfig) Validate() error {
	if bc.svc == nil {
		return errors.New("service configuration is required")
	}
	if bc.svc.port == 0 {
		bc.svc.port = 8080 // Default port
	}
	for _, s := range bc.sources {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("source %s: %w", s.nam, err)
		}
	}
	return nil
}

// Implement boiler.Config interface
func (bc *BaseConfig) Sources() []boiler.IOWithAuth {
	result := make([]boiler.IOWithAuth, len(bc.sources))
	for i, src := range bc.sources {
		result[i] = src
	}
	return result
}

// Getter methods for BaseConfig
func (bc *BaseConfig) Service() *service    { return bc.svc }
func (bc *BaseConfig) LogConfig() logConfig { return bc.log }
func (bc *BaseConfig) Extension() any       { return bc.ext }

type service struct {
	name    string `yaml:"name" env:"SERVICE_NAME"`
	purpose string `yaml:"purpose" env:"SERVICE_PURPOSE"`
	port    int    `yaml:"port" env:"SERVICE_PORT" default:"8080"`
}

type source struct {
	nam   string       `yaml:"name"`
	typ   string       `yaml:"type"`
	enbl  bool         `yaml:"enabled"`
	loc   string       `yaml:"location"`
	creds *credentials `yaml:"auth,omitempty"`
}

func (s *source) Validate() error {
	if s.nam == "" {
		return errors.New("name is required")
	}
	if s.loc == "" {
		return errors.New("location is required")
	}
	if s.creds != nil {
		if err := s.creds.Validate(); err != nil {
			return fmt.Errorf("credentials error: %w", err)
		}
	}
	return nil
}

// Implement boiler.IOWithAuth interface
func (s source) Auth() boiler.Credentials {
	if s.creds == nil {
		return credentials{} // Return empty credentials if none configured
	}
	return *s.creds
}
func (s source) Enabled() bool { return s.enbl }
func (s source) Type() string  { return s.typ }
func (s source) Name() string  { return s.nam }
func (s source) Addr() string  { return s.loc }

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
	if c.ext == nil {
		return result, errors.New("no extension configuration found")
	}

	yamlData, err := yaml.Marshal(c.ext)
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
