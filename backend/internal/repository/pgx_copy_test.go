package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type stubPGXConnectionProvider struct {
	conn *pgx.Conn
}

func (p stubPGXConnectionProvider) Conn() *pgx.Conn {
	return p.conn
}

func (p stubPGXConnectionProvider) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (p stubPGXConnectionProvider) Close() error {
	return nil
}

func (p stubPGXConnectionProvider) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func TestUnwrapPGXConnection(t *testing.T) {
	want := &pgx.Conn{}

	direct, instrumented := unwrapPGXConnection(stubPGXConnectionProvider{conn: want})
	require.Same(t, want, direct)
	require.False(t, instrumented)

	wrapped, instrumented := unwrapPGXConnection(&serverTimingConn{Conn: stubPGXConnectionProvider{conn: want}})
	require.Same(t, want, wrapped)
	require.True(t, instrumented)
}

func TestCopyFromSQLDBEmptyBatchDoesNotAcquireConnection(t *testing.T) {
	copied, err := copyFromSQLDB(context.Background(), nil, nil, nil, nil)
	require.NoError(t, err)
	require.Zero(t, copied)
}
