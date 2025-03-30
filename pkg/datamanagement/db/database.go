package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"reflect"
)

var ErrBadConfig = errors.New("the configuration provided is missing fields or has bad values in the provided fields")

type dbMode int

type QueryConstructor interface {
	Construct() string
}

type QueryWrapper interface {
	Wrap(*sql.Rows)
}

type QueryUnwrapper interface {
	Unwrap() any
}

type Query interface {
	QueryConstructor
	QueryWrapper
	QueryUnwrapper
}

const (
	stage dbMode = iota
	prod  dbMode = iota
)

// DatabaseConfig provides the necessary configuration for Database initiailization; All fields must be filled
type DatabaseConfig struct {
	Driver      string `json:"driver"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Credentials struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	} `json:"credentials"`
	/* 	 ConnectionStringTemplate example:"sqlserver://{{.Credentials.Name}}:{{.Credentials.Password}}@{{.Address}}/?database={{.Name}}" */
	ConnectionStringTemplate *template.Template
}

type Database struct {
	Config     DatabaseConfig
	db         *sql.DB
	connString string
	prepStmts  map[string]*sql.Stmt
	open       bool
}

func ValidateConfig(c DatabaseConfig) error {
	valid := len(c.Address) != 0 && len(c.Driver) != 0 && c.ConnectionStringTemplate != nil && len(c.Credentials.Name) != 0 && len(c.Credentials.Password) != 0
	if !valid {
		return ErrBadConfig
	}
	return nil
}

func NewDatabase(c DatabaseConfig, name string) (*Database, error) {
	if err := ValidateConfig(c); err != nil {
		return nil, err
	}
	connectionString := bytes.NewBuffer([]byte{})
	db := new(Database)
	db.Config = c
	err := db.Config.ConnectionStringTemplate.Execute(connectionString, db.Config)
	if err != nil {
		return nil, err
	}
	db.connString = connectionString.String()

	return nil, errors.New("no compatible source found")
}

func (pdb *Database) Open() error {
	var err error
	pdb.db, err = sql.Open(pdb.Config.Driver, pdb.connString)
	if err != nil {
		return err
	}
	pdb.open = true
	pdb.prepStmts = make(map[string]*sql.Stmt)
	return nil
}

func (pdb *Database) Close() error {
	err := pdb.db.Close()
	if err != nil {
		return err
	}
	pdb.open = false
	return nil
}

func (pdb *Database) Query(qc Query, params ...any) (QueryUnwrapper, error) {
	var stmt *sql.Stmt
	var ok bool
	var err error
	err = pdb.Open()
	defer pdb.Close()
	if err != nil {
		return nil, err
	}
	defer pdb.Close()
	if stmt, ok = pdb.prepStmts[reflect.TypeOf(qc).Name()]; !ok {
		stmt, err = pdb.db.Prepare(qc.Construct())
		if err != nil {
			return nil, err
		}
		pdb.prepStmts[reflect.TypeOf(qc).Name()] = stmt

	}
	q, err := stmt.Query(params...)
	if err != nil {
		return nil, err
	}
	qc.Wrap(q)
	return qc, nil
}

func (pdb *Database) Execute(qc QueryConstructor, params ...any) (sql.Result, error) {
	var stmt *sql.Stmt
	var ok bool
	var err error
	err = pdb.Open()
	defer pdb.Close()
	if err != nil {
		return nil, err
	}
	defer pdb.Close()
	if stmt, ok = pdb.prepStmts[reflect.TypeOf(qc).Name()]; !ok {
		stmt, err = pdb.db.Prepare(qc.Construct())
		if err != nil {
			return nil, fmt.Errorf("statement construction error:%w", err)
		}
		pdb.prepStmts[reflect.TypeOf(qc).Name()] = stmt
	}
	return stmt.Exec(params...)
}
