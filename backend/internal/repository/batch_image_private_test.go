package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBatchImageRepositoryPrivateOperatorCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &batchImageRepository{sql: db}

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	allowed, err := repo.IsPrivateBatchImageOperator(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}
