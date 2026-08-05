package server

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestProbeReadiness(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		redisServer := miniredis.RunT(t)
		redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		defer redisClient.Close()

		require.NoError(t, probeReadiness(context.Background(), db, redisClient))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_unavailable", func(t *testing.T) {
		require.EqualError(t, probeReadiness(context.Background(), nil, nil), "database unavailable")
	})

	t.Run("database_ping_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing().WillReturnError(errors.New("postgres unavailable"))

		require.ErrorContains(t, probeReadiness(context.Background(), db, nil), "database ping")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("migration_table_missing", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").WillReturnError(errors.New("relation does not exist"))

		require.ErrorContains(t, probeReadiness(context.Background(), db, nil), "schema migrations check")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no_migration_applied", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		require.EqualError(t, probeReadiness(context.Background(), db, nil), "schema migrations unavailable")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("redis_unavailable", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		require.EqualError(t, probeReadiness(context.Background(), db, nil), "redis unavailable")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
