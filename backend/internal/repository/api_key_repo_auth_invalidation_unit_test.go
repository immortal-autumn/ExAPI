//go:build unit

package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepository_HasPendingAuthCacheInvalidationQueriesDurableOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT EXISTS\s*\(\s*SELECT 1\s*FROM auth_cache_invalidation_outbox`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	repo := newAPIKeyRepositoryWithSQL(nil, db)
	pending, err := repo.HasPendingAuthCacheInvalidation(context.Background())
	require.NoError(t, err)
	require.True(t, pending)
	require.NoError(t, mock.ExpectationsWereMet())
}
