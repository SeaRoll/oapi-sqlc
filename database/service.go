package database

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	// Import the pgx driver stdlib.
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Database defines the public interface for dbo.
type Database interface {
	// Disconnect closes the database connection pool and sets the teardown flag to true.
	// This method should be called when the application is shutting down to ensure all resources are released properly.
	// It sets the isTeardown flag to true to indicate that the database connection is being torn down.
	// This prevents any further operations on the database connection pool after it has been closed.
	//
	// If `noTeardown` is true, it will not set the teardown flag,
	// allowing the client to be reused later.
	// // If `noTeardown` is false or not provided, it will set the teardown flag
	// and close the client connection, preventing any further operations.
	Disconnect(noTeardown ...bool)
	// WithReadTX executes a function within a read-only database transaction context.
	// If an existing Querier is provided via existingQ, it uses that instead of creating a new transaction.
	// Otherwise, it begins a new read-only transaction, executes the provided function with the transaction-aware querier,
	// and commits the transaction on success or rolls back on error.
	// The function automatically handles transaction cleanup through deferred rollback.
	// This method is optimized for read operations and may provide better performance for queries that don't modify data.
	//
	// Parameters:
	// - ctx: Context for the transaction operation
	// - fn: Function to execute within the transaction, receives a Querier interface
	// - existingQ: Optional existing Querier to reuse instead of creating a new transaction
	//
	// Returns:
	// - error: Any error from transaction operations or the executed function
	WithReadTX(ctx context.Context, fn func(Querier) error, existingQ ...Querier) error
	// WithTX executes a function within a database transaction context.
	// If an existing Querier is provided via existingQ, it uses that instead of creating a new transaction.
	// Otherwise, it begins a new read-write transaction, executes the provided function with the transaction-aware querier,
	// and commits the transaction on success or rolls back on error.
	// The function automatically handles transaction cleanup through deferred rollback.
	//
	// Parameters:
	// - ctx: Context for the transaction operation
	// - fn: Function to execute within the transaction, receives a Querier interface
	// - existingQ: Optional existing Querier to reuse instead of creating a new transaction
	//
	// Returns:
	// - error: Any error from transaction operations or the executed function
	WithTX(ctx context.Context, fn func(Querier) error, existingQ ...Querier) error
}

//go:generate rm -f *.sql.go
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc generate --file ./sqlc.yaml

type dbo struct {
	url        string
	pool       *pgxpool.Pool
	isTeardown atomic.Bool
}

func NewDatabase(url string) Database {
	d := &dbo{
		url:        url,
		isTeardown: atomic.Bool{},
	}

	err := d.connectAndMigratePool()
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		panic(err)
	}

	d.runReconnect()

	return d
}

// runReconnect starts a goroutine that periodically checks the health of the database connection pool.
func (d *dbo) runReconnect() {
	// create go coroutine to check every dbs health
	// if any db is not healthy, try to reconnect
	go func() {
		for {
			// check if tearing down is requested
			if d.isTeardown.Load() {
				slog.Info("db is being torn down, skipping health check")
				return
			}

			d.healthCheckPool()
			time.Sleep(5 * time.Second)
		}
	}()
}

// healthCheckPool checks the health of the database connection pool.
func (d *dbo) healthCheckPool() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := d.pool.Ping(ctx)
	if err != nil {
		slog.Error("db is not healthy", "error", err)

		err := d.connectAndMigratePool()
		if err != nil {
			slog.Error("failed to reconnect to db", "error", err)
		} else {
			slog.Info("reconnected to db")
		}
	}
}

// connectAndMigratePool connects to the database and runs migrations.
// It constructs the database URL from the configuration, parses it, and creates a connection pool.
func (d *dbo) connectAndMigratePool() error {
	// give 15 seconds for the db to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(d.url)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	dbo := stdlib.OpenDBFromPool(pool)

	err = migrate(dbo)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	d.pool = pool

	return nil
}

// Disconnect closes the database connection pool and sets the teardown flag to true.
// This method should be called when the application is shutting down to ensure all resources are released properly.
// It sets the isTeardown flag to true to indicate that the database connection is being torn down.
// This prevents any further operations on the database connection pool after it has been closed.
//
// If `noTeardown` is true, it will not set the teardown flag,
// allowing the client to be reused later.
// // If `noTeardown` is false or not provided, it will set the teardown flag
// and close the client connection, preventing any further operations.
func (d *dbo) Disconnect(noTeardown ...bool) {
	if len(noTeardown) == 0 || !noTeardown[0] {
		d.isTeardown.Store(true) // Set teardown flag to true
	}

	d.pool.Close()
}

// WithReadTX executes a function within a read-only database transaction context.
// If an existing Querier is provided via existingQ, it uses that instead of creating a new transaction.
// Otherwise, it begins a new read-only transaction, executes the provided function with the transaction-aware querier,
// and commits the transaction on success or rolls back on error.
// The function automatically handles transaction cleanup through deferred rollback.
// This method is optimized for read operations and may provide better performance for queries that don't modify data.
//
// Parameters:
//   - ctx: Context for the transaction operation
//   - fn: Function to execute within the transaction, receives a Querier interface
//   - existingQ: Optional existing Querier to reuse instead of creating a new transaction
//
// Returns:
//   - error: Any error from transaction operations or the executed function
func (d *dbo) WithReadTX(ctx context.Context, fn func(Querier) error, existingQ ...Querier) error {
	return d.runTransactionWithOpts(ctx, fn, pgx.TxOptions{AccessMode: pgx.ReadOnly}, existingQ...)
}

// WithTX executes a function within a database transaction context.
// If an existing Querier is provided via existingQ, it uses that instead of creating a new transaction.
// Otherwise, it begins a new read-write transaction, executes the provided function with the transaction-aware querier,
// and commits the transaction on success or rolls back on error.
// The function automatically handles transaction cleanup through deferred rollback.
//
// Parameters:
//   - ctx: Context for the transaction operation
//   - fn: Function to execute within the transaction, receives a Querier interface
//   - existingQ: Optional existing Querier to reuse instead of creating a new transaction
//
// Returns:
//   - error: Any error from transaction operations or the executed function
func (d *dbo) WithTX(ctx context.Context, fn func(Querier) error, existingQ ...Querier) error {
	return d.runTransactionWithOpts(ctx, fn, pgx.TxOptions{AccessMode: pgx.ReadWrite}, existingQ...)
}

// runTransactionWithOpts executes a function within a transaction context with specified options.
// If an existing Querier is provided via existingQ, it uses that instead of creating a new transaction.
// Otherwise, it begins a new transaction with the provided options, executes the function with the transaction-aware querier,
// and commits the transaction on success or rolls back on error.
// The function automatically handles transaction cleanup through deferred rollback.
//
// Parameters:
//   - ctx: Context for the transaction operation
//   - fn: Function to execute within the transaction, receives a Querier interface
//   - opts: Transaction options to configure the transaction behavior
//   - existingQ: Optional existing Querier to reuse instead of creating a new transaction
//
// Returns:
//   - error: Any error from transaction operations or the executed function
func (d *dbo) runTransactionWithOpts(ctx context.Context, fn func(Querier) error, opts pgx.TxOptions, existingQ ...Querier) error {
	if len(existingQ) > 0 {
		return fn(existingQ[0])
	}

	queries := New(d.pool)

	tx, err := d.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = fn(queries.WithTx(tx))
	if err != nil {
		return fmt.Errorf("failed to execute function: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
