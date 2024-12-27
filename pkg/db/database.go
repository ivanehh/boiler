package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ivanehh/boiler"
	_ "github.com/lib/pq"               // PostgreSQL
	_ "github.com/mattn/go-sqlite3"     // SQLite
	_ "github.com/microsoft/go-mssqldb" // MSSQL
)

// Common errors
var (
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
type Config struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig provides sensible defaults for database configuration
func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

var dbConfig = DefaultConfig()

func ChangeConfig(db *DB, c Config) {
	dbConfig = c
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)
}

// New creates a new database connection
func New(source boiler.IOWithAuth) (*DB, error) {
	connString := buildConnString(source)

	db := &DB{
		driver:     source.Type(),
		connString: connString,
	}

	if err := db.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	// Configure connection pool
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)
	return db, nil
}

// connect establishes the database connection and configures the connection pool
func (db *DB) connect() error {
	// TODO: Currently the config does nothing upon attempting connectiong
	var err error
	db.DB, err = sql.Open(db.driver, db.connString)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	// Test connection with context and timeout
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

// buildConnString creates a connection string from the source configuration
func buildConnString(source boiler.IOWithAuth) string {
	switch source.Type() {
	case "postgres":
		return fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable",
			source.Auth().Username(),
			source.Auth().Password(),
			source.Addr(),
			source.Name())
	case "mssql":
		// Format: sqlserver://username:password@host/database?param1=value&param2=value
		return fmt.Sprintf("sqlserver://%s:%s@%s?database=%s",
			source.Auth().Username(),
			source.Auth().Password(),
			source.Addr(),
			source.Name())

	case "mysql":
		// Format: username:password@tcp(host:port)/database
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
			source.Auth().Username(),
			source.Auth().Password(),
			source.Addr(),
			source.Name())
	default:
		return ""
	}
}
