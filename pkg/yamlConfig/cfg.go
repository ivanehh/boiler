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

type ConfigManager struct {
	current *BaseConfig
	mu      sync.RWMutex
}

var manager = &ConfigManager{}

type BaseConfig struct {
	svc *service  `yaml:"service"`
	io  dataIO    `yaml:"sources"`
	log logConfig `yaml:"logging"`
	ext any       `yaml:"extension,omitempty"`
}

func (bc *BaseConfig) Validate() error {
	if bc.svc == nil {
		return errors.New("service configuration is required")
	}
	if bc.svc.port == 0 {
		bc.svc.port = 8080 // Default port
	}
	if err := bc.io.Validate(); err != nil {
		return fmt.Errorf("IO configuration error: %w", err)
	}
	return nil
}

type service struct {
	name    string `yaml:"name" env:"SERVICE_NAME"`
	purpose string `yaml:"purpose" env:"SERVICE_PURPOSE"`
	port    int    `yaml:"port" env:"SERVICE_PORT" default:"8080"`
}

type dataIO struct {
	db    []secureSource   `yaml:"databases"`
	http  []secureSource   `yaml:"http"`
	other []insecureSource `yaml:"other"`
}

func (d *dataIO) Validate() error {
	for _, db := range d.db {
		if err := db.Validate(); err != nil {
			return fmt.Errorf("database %s: %w", db.nam, err)
		}
	}
	return nil
}

func (s dataIO) Databases() []boiler.IOWithAuth {
	dbs := make([]boiler.IOWithAuth, len(s.db))
	for idx, d := range s.db {
		dbs[idx] = d
	}
	return dbs
}

func (s dataIO) HTTPs() []boiler.IOWithAuth {
	https := make([]boiler.IOWithAuth, len(s.http))
	for idx, f := range s.http {
		https[idx] = f
	}
	return https
}

func (s dataIO) Others() []boiler.IONoAuth {
	others := make([]boiler.IONoAuth, len(s.other))
	for idx, f := range s.other {
		others[idx] = f
	}
	return others
}

type secureSource struct {
	nam   string      `yaml:"name"`
	typ   string      `yaml:"type"`
	enbl  bool        `yaml:"enabled"`
	loc   string      `yaml:"location"`
	creds credentials `yaml:"auth"`
}

func (s *secureSource) Validate() error {
	if s.nam == "" {
		return errors.New("name is required")
	}
	if s.loc == "" {
		return errors.New("location is required")
	}
	if s.enbl {
		if err := s.creds.Validate(); err != nil {
			return fmt.Errorf("credentials error: %w", err)
		}
	}
	return nil
}

// Implement boiler.IOWithAuth interface
func (s secureSource) Auth() boiler.Credentials { return s.creds }
func (s secureSource) Enabled() bool            { return s.enbl }
func (s secureSource) Type() string             { return s.typ }
func (s secureSource) Name() string             { return s.nam }
func (s secureSource) Addr() string             { return s.loc }

type insecureSource struct {
	nam  string   `yaml:"name"`
	typ  []string `yaml:"type"`
	enbl bool     `yaml:"enabled"`
	loc  string   `yaml:"location"`
}

// Implement boiler.IONoAuth interface
func (s insecureSource) Enabled() bool  { return s.enbl }
func (s insecureSource) Type() []string { return s.typ }
func (s insecureSource) Name() string   { return s.nam }
func (s insecureSource) Addr() string   { return s.loc }

type credentials struct {
	uname string `yaml:"username"`
	pwd   string `yaml:"password"`
}

func (c *credentials) Validate() error {
	if c.uname == "" {
		return errors.New("username is required for secure sources")
	}
	if c.pwd == "" {
		return errors.New("password is required for secure sources")
	}
	return nil
}

// Implement boiler.Credentials interface
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

// ExtensionAs converts the generic extension configuration to a specific type
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

// Load reads and parses the configuration file
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

// Reload refreshes the configuration from disk
func Reload() error {
	return Load("")
}

// Get returns the current configuration in a thread-safe manner
func Get() *BaseConfig {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if manager.current == nil {
		panic("configuration must be loaded before use")
	}
	return manager.current
}

// Getter methods for BaseConfig
func (bc *BaseConfig) Service() *service       { return bc.svc }
func (bc *BaseConfig) Sources() boiler.Sources { return bc.io }
func (bc *BaseConfig) LogConfig() logConfig    { return bc.log }
func (bc *BaseConfig) Extension() any          { return bc.ext }
