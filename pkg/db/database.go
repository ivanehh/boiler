package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strings"

	pkg "github.com/ivanehh/boiler/pkg"
)

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

var conStr func(s pkg.IOWithAuth) string = func(s pkg.IOWithAuth) string {
	auth := s.Auth()
	connectData := struct {
		Username string
		Password string
		Location string
		Name     string
	}{
		Username: auth.Username(),
		Password: auth.Password(),
		Location: s.Addr(),
		Name:     s.Name(),
	}
	out := &bytes.Buffer{}
	tm := template.Must(template.New("connstring").Parse("sqlserver://{{.Username}}:{{.Password}}@{{.Location}}/?database={{.Name}}"))
	tm.Execute(out, connectData)
	return out.String()
}

type Database struct {
	db         *sql.DB
	connString string
	prepStmts  map[string]*sql.Stmt
	open       bool
}

func NewDatabase(c pkg.Config, name string) (*Database, error) {
	var src pkg.IOWithAuth
	var db *Database
	for _, source := range c.Sources().Databases() {
		if source.Enabled() && source.Type() == "mssql" {
			if strings.Contains(source.Name(), name) {
				src = source
				db = &Database{
					connString: conStr(src),
					prepStmts:  make(map[string]*sql.Stmt),
				}
				fmt.Printf("db: %+v\n", db)
				return db, nil
			}
		}
	}
	return nil, errors.New("no compatible source found")
}

func (pdb *Database) Open() error {
	var err error
	pdb.db, err = sql.Open("mssql", pdb.connString)
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
