package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"               // PostgreSQL
	_ "github.com/mattn/go-sqlite3"     // SQLite
	_ "github.com/microsoft/go-mssqldb" // MSSQL
)

// Common errors
var (
	ErrBadConfig    = errors.New("the configuration provided is missing fields or has bad values in the provided fields")
	ErrNoConnection = errors.New("database connection not initialized")
	ErrNoRows       = sql.ErrNoRows
)

// DB wraps sql.DB to provide additional functionality
type DB struct {
	*sql.DB
	driver     string
	connString string
}

// Config represents database configuration
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

// New creates a new database connection
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

	return db, nil
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

type Scanner[T any] func(*sql.Rows) (T, error)

type SingleRowScanner[T any] func(*sql.Row) (T, error)

func QueryRow[T any](ctx context.Context, db *DB, query string, scanner SingleRowScanner[T], args ...any) (T, error) {
	if db.DB == nil {
		var zero T
		return zero, ErrNoConnection
	}

	row := db.QueryRowContext(ctx, query, args...)
	return scanner(row)
}

func QueryRows[T any](ctx context.Context, db *DB, query string, scanner Scanner[T], args ...any) ([]T, error) {
	if db.DB == nil {
		return nil, ErrNoConnection
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		result, err := scanner(rows)
		if err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// Exec executes a query that doesn't return rows (INSERT, UPDATE, DELETE)
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db.DB == nil {
		return nil, ErrNoConnection
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	return result, nil
}

func (db *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	if db.DB == nil {
		return ErrNoConnection
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error: %v, rollback error: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Health performs a health check on the database
func (db *DB) Health(ctx context.Context) error {
	if db.DB == nil {
		return ErrNoConnection
	}

	return db.PingContext(ctx)
}
