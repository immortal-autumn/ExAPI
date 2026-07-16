package repository

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type failingReadCloser struct {
	err error
}

func (r *failingReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r *failingReadCloser) Close() error             { return nil }

func TestPgDumpReadCloserPropagatesProducerReadErrorWithoutWaiting(t *testing.T) {
	producerErr := errors.New("pg_dump stdout failed")
	reader := &cmdReadCloser{ReadCloser: &failingReadCloser{err: producerErr}}

	_, err := io.ReadAll(reader)
	require.ErrorIs(t, err, producerErr)
	require.NoError(t, reader.Close())
}

func TestPgDumperRestoreEnforcesStopOnError(t *testing.T) {
	d := &PgDumper{cfg: &config.DatabaseConfig{Host: "db", Port: 5432, User: "user", DBName: "name"}}
	var captured []string
	d.command = func(_ context.Context, _ string, args ...string) *exec.Cmd {
		captured = append([]string(nil), args...)
		return exec.Command("sh", "-c", "cat >/dev/null; exit 0")
	}

	require.NoError(t, d.Restore(context.Background(), strings.NewReader("SELECT 1;")))
	require.Contains(t, captured, "ON_ERROR_STOP=on")
	for i, arg := range captured {
		if arg == "--set" {
			require.Less(t, i+1, len(captured))
			require.Equal(t, "ON_ERROR_STOP=on", captured[i+1])
			return
		}
	}
	t.Fatal("missing --set argument")
}

func TestPgDumperRestoreRedactsCommandOutput(t *testing.T) {
	d := &PgDumper{
		cfg: &config.DatabaseConfig{Host: "127.0.0.1", Port: 1, User: "test", DBName: "test"},
		command: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", "cat >/dev/null; printf 'server error containing supplied dump' >&2; exit 1")
		},
	}

	err := d.Restore(context.Background(), strings.NewReader("secret SQL content"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "supplied dump")
	require.NotContains(t, err.Error(), "secret SQL")
}
