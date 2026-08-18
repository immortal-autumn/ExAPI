package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/jackc/pgx/v5"
)

type pgxConnectionProvider interface {
	Conn() *pgx.Conn
}

func copyFromSQLDB(
	ctx context.Context,
	db *sql.DB,
	table pgx.Identifier,
	columns []string,
	rows [][]any,
) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	var copied int64
	err = conn.Raw(func(driverConn any) error {
		pgxConn, instrumented := unwrapPGXConnection(driverConn)
		if pgxConn == nil {
			return fmt.Errorf("PostgreSQL COPY requires pgx stdlib connection, got %T", driverConn)
		}
		startedAt := time.Now()
		var copyErr error
		copied, copyErr = pgxConn.CopyFrom(ctx, table, columns, pgx.CopyFromRows(rows))
		if instrumented {
			servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, time.Now(), 1)
		}
		return copyErr
	})
	return copied, err
}

func unwrapPGXConnection(driverConn any) (*pgx.Conn, bool) {
	if provider, ok := driverConn.(pgxConnectionProvider); ok {
		return provider.Conn(), false
	}
	if wrapped, ok := driverConn.(*serverTimingConn); ok {
		conn, _ := unwrapPGXConnection(wrapped.Conn)
		return conn, true
	}
	return nil, false
}
