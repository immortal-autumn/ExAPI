package postgres

import (
	"database/sql/driver"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestNewConnectorAcceptsApplicationKeywordDSN(t *testing.T) {
	connector, err := NewConnector("host=127.0.0.1 port=5432 user=postgres password=secret dbname=sub2api sslmode=disable TimeZone=UTC")
	require.NoError(t, err)
	require.NotNil(t, connector)
	require.True(t, IsDriver(connector.Driver()))
}

func TestArraySupportsPGXAndGenericDatabaseSQLDrivers(t *testing.T) {
	value := Array([]string{"comma,value", "quote\"value", "slash\\value"})
	_, ok := value.(pgtype.ArrayGetter)
	require.True(t, ok)

	valuer, ok := value.(driver.Valuer)
	require.True(t, ok)
	encoded, err := valuer.Value()
	require.NoError(t, err)
	require.Equal(t, `{"comma,value","quote\"value","slash\\value"}`, encoded)
}

func TestSQLStateHandlesWrappedAndTypedNilErrors(t *testing.T) {
	require.Equal(t, "23505", SQLState(fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"})))
	require.True(t, IsSQLState(&pgconn.PgError{Code: "23503"}, "23503"))

	var typedNil *pgconn.PgError
	require.Empty(t, SQLState(typedNil))
}

func TestQuoteHelpers(t *testing.T) {
	require.Equal(t, `"partition""name"`, QuoteIdentifier(`partition"name`))
	require.Equal(t, `'2026-01-01''UTC'`, QuoteLiteral(`2026-01-01'UTC`))
}
