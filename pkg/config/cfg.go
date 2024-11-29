package config

/*
Defines config importer and provides it to rest of application
*/

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ivanehh/boiler"
	"github.com/ivanehh/boiler/internal/helpers"
	"gopkg.in/yaml.v3"
)

var baseC *BaseConfig

type BaseConfig struct {
	Svc    *service  `yaml:"service"`
	Src    dataIO    `yaml:"sources"`
	Dstns  dataIO    `yaml:"destinations"`
	Log    logConfig `yaml:"logging"`
	Ext    any       `yaml:"extension,omitempty"`
	loaded bool
}

func (bc BaseConfig) Service() service {
	return *bc.Svc
}

func (bc BaseConfig) Sources() boiler.Sources {
	return bc.Src
}

func (bc BaseConfig) Destinations() dataIO {
	return bc.Dstns
}

func (bc BaseConfig) LogConfig() logConfig {
	return bc.Log
}

func (bc BaseConfig) Extension() any {
	return bc.Ext
}

/*  ExtensionAs does a simple type conversion for the generically consumed ConfigurationExtension */
func ExtensionAs[T any](c *BaseConfig) (T, error) {
	var trgtExt T
	var extSrc map[string]any
	var ok bool
	if extSrc, ok = (baseC.Extension()).(map[string]any); !ok {
		return trgtExt, fmt.Errorf("the provided extension is not of type %T", extSrc)
	}
	yamlData, err := yaml.Marshal(c.Extension())
	if err != nil {
		return trgtExt, err
	}
	err = yaml.Unmarshal(yamlData, &trgtExt)
	if err != nil {
		return trgtExt, err
	}
	return trgtExt, nil
}

type service struct {
	Name    string `yaml:"name,omitempty"`
	Purpose string `yaml:"purpose,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

type destination struct {
	Name      string      `yaml:"name"`
	Type      string      `yaml:"type"`
	Location  string      `yaml:"location"`
	Port      string      `yaml:"port,omitempty"`
	Enabled   bool        `yaml:"enabled"`
	Endpoints []string    `yaml:"endpoints,omitempty"`
	Auth      credentials `yaml:"auth,omitempty"`
}

type dataIO struct {
	Db   []dbSource   `yaml:"databases"`
	Ftp  []ftpSource  `yaml:"ftp"`
	Http []httpSource `yaml:"http"`
}

func (s dataIO) Databases() []boiler.IOWithAuth {
	dbs := make([]boiler.IOWithAuth, len(s.Db))
	for idx, d := range s.Db {
		dbs[idx] = d
	}
	return dbs
}

func (s dataIO) FTPs() []boiler.IONoAuth {
	ftps := make([]boiler.IONoAuth, len(s.Ftp))
	for idx, f := range s.Ftp {
		ftps[idx] = f
	}
	return ftps
}

func (s dataIO) HTTPs() []boiler.IONoAuth {
	https := make([]boiler.IONoAuth, len(s.Http))
	for idx, f := range s.Http {
		https[idx] = f
	}
	return https
}

type dbSource struct {
	Nam   string      `yaml:"name,omitempty"`
	Typ   string      `yaml:"type,omitempty"`
	Enbl  bool        `yaml:"enabled,omitempty"`
	Loc   string      `yaml:"location,omitempty"`
	Rfrsh int         `yaml:"refresh,omitempty"`
	Creds credentials `yaml:"auth,omitempty"`
}

func (s dbSource) Enabled() bool {
	return s.Enbl
}

func (s dbSource) Type() string {
	return s.Typ
}

func (s dbSource) Name() string {
	return s.Nam
}

func (s dbSource) Addr() string {
	return s.Loc
}

func (s dbSource) Auth() boiler.Credentials {
	return s.Creds
}

type httpSource struct {
	Nam  string   `yaml:"name,omitempty"`
	Typ  []string `yaml:"type,omitempty"`
	Enbl bool     `yaml:"enabled,omitempty"`
	Loc  string   `yaml:"location,omitempty"`
}

func (s httpSource) Enabled() bool {
	return s.Enbl
}

func (s httpSource) Type() []string {
	return s.Typ
}

func (s httpSource) Name() string {
	return s.Nam
}

func (s httpSource) Addr() string {
	return s.Loc
}

type ftpSource struct {
	Nam  string   `yaml:"name,omitempty"`
	Typ  []string `yaml:"type,omitempty"`
	Enbl bool     `yaml:"enabled,omitempty"`
	Loc  string   `yaml:"location,omitempty"`
}

func (s ftpSource) Enabled() bool {
	return s.Enbl
}

func (s ftpSource) Type() []string {
	return s.Typ
}

func (s ftpSource) Name() string {
	return s.Nam
}

func (s ftpSource) Addr() string {
	return s.Loc
}

type credentials struct {
	Uname string `yaml:"username,omitempty"`
	Pwd   string `yaml:"password,omitempty"`
}

func (crd credentials) Username() string {
	return crd.Uname
}

func (crd credentials) Password() string {
	return crd.Pwd
}

// NOTE: The configuration might not work for plugging loggers into workplaces
type logConfig struct {
	Level   string `yaml:"level" json:"level,omitempty"`
	Folder  string `yaml:"filePath" json:"file_path,omitempty"`
	MaxSize int    `yaml:"maxSize" json:"max_size,omitempty"` // MaxFiles  int    `yaml:"maxFiles"`
}

func (lc logConfig) MinLevel() slog.Level {
	switch strings.ToLower(lc.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		fmt.Print("log level not recognized; returning LevelInfo")
		return slog.LevelInfo
	}
}

func (lc logConfig) Dir() string {
	return lc.Folder
}

func (lc logConfig) MaxFileSize() int {
	return lc.MaxSize
}

/*
Load provides a BaseConfig either by

- calculating the root path based on a hardcoded pattern (see implementation)

- using the provided override; the override must be only 1 string argument; if more than 1 argument is provided then Load returns an empty BaseConfig and an error
*/
func Load(override string) error {
	baseC = &BaseConfig{}
	fp := override
	if len(override) == 0 {
		fp = filepath.Join(helpers.Rootpath(), "config", "cfg.yaml")
	}
	if !filepath.IsAbs(fp) {
		fp = "/" + fp
	}
	yamlFile, err := os.ReadFile(fp)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &baseC)
	if err != nil {
		return err
	}
	return nil
}

// Provides an already loaded BaseConfig; panics if configuration hasn't been loaded
func Provide() *BaseConfig {
	if baseC == nil {
		panic(errors.New("base configuration must be loaded first"))
	}
	return baseC
}
