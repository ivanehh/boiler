package yamlConfig

import (
	"flag"
	"io"

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

type Config[B any, E any] struct {
	Base  B
	Flags flag.FlagSet
	Env   E
}

func NewConfig[B any, E any]() *Config[B, E] {
	return &Config[B, E]{
		Base:  *new(B),
		Flags: flag.FlagSet{},
		Env:   *new(E),
	}
}

// Yaml only
func (c *Config[B, E]) ParseBase(r io.Reader) error {
	err := yaml.NewDecoder(r).Decode(c.Base)
	if err != nil {
		return err
	}
	return nil
}
