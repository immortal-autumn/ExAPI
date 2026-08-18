//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestPGXDatabaseSQLCompatibility(t *testing.T) {
	ctx := context.Background()

	var cardinality int
	require.NoError(t, integrationDB.QueryRowContext(
		ctx,
		`SELECT cardinality($1::bigint[])`,
		postgres.Array([]int64{11, 12, 13}),
	).Scan(&cardinality))
	require.Equal(t, 3, cardinality)

	var scanned []int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT ARRAY[21,22]::bigint[]`).Scan(postgres.Array(&scanned)))
	require.Equal(t, []int64{21, 22}, scanned)

	const table = "pgx_database_sql_compatibility"
	_, err := integrationDB.ExecContext(ctx, `DROP TABLE IF EXISTS `+postgres.QuoteIdentifier(table))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `CREATE TABLE `+postgres.QuoteIdentifier(table)+` (id BIGINT PRIMARY KEY, payload JSONB NOT NULL)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DROP TABLE IF EXISTS `+postgres.QuoteIdentifier(table))
	})

	copied, err := copyFromSQLDB(ctx, integrationDB, pgx.Identifier{table}, []string{"id", "payload"}, [][]any{
		{int64(1), `{"source":"copy"}`},
		{int64(2), `{"source":"copy"}`},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, copied)

	_, err = integrationDB.ExecContext(ctx, `INSERT INTO `+postgres.QuoteIdentifier(table)+` (id, payload) VALUES ($1, $2)`, int64(1), `{}`)
	require.True(t, postgres.IsSQLState(err, "23505"), "expected pgx SQLSTATE 23505, got %v", err)
}
