package dbx

import (
	"context"
	"log"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/tantra/auth/session"
	"github.com/mudgallabs/tantra/logger"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Init(url string) (*pgxpool.Pool, error) {
	l := logger.Get()

	l.Info("connecting to database")

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Panic(err)
		return nil, err
	}

	// So that we can log SQL query on execution.
	config.ConnConfig.Tracer = &myQueryTracer{}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Register `pgxdecimal` so that we can use `decimal.Decimal` for values while scaning or inserting records.
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Panic(err)
		return nil, err
	}

	session.Manager.Store = pgxstore.NewWithCleanupInterval(pool, 12*time.Hour)

	// Checking if the connection to the DB is working fine.
	err = pool.Ping(context.Background())
	if err != nil {
		log.Panic(err)
		return nil, err
	}

	l.Info("connected to database")

	return pool, nil
}

type myQueryTracer struct {
}

func (tracer *myQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	l := logger.FromCtx(ctx)
	l.Debugw("executing SQL query", "sqlstr", data.SQL, "args", data.Args)
	return ctx
}

func (tracer *myQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		l := logger.FromCtx(ctx)
		l.Debugw("error executing SQL query", "err", data.Err)
	}
}

// WithTx runs fn within a transaction.
// It commits if fn returns nil, or rolls back if fn returns an error or panics.
func WithTx(ctx context.Context, db *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	l := logger.FromCtx(ctx)
	start := time.Now()

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		duration := time.Since(start)
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			l.Debugf("[tx] panic after %s: %v", duration, p)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
			l.Debugf("[tx] rolled back after %s: %v", duration, err)
		} else {
			err = tx.Commit(ctx)
			l.Debugf("[tx] committed after %s", duration)
		}
	}()

	err = fn(tx)
	return
}
