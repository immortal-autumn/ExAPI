package repository

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// PgDumper implements service.DBDumper using pg_dump/psql
type PgDumper struct {
	cfg     *config.DatabaseConfig
	command func(context.Context, string, ...string) *exec.Cmd
}

// NewPgDumper creates a new PgDumper
func NewPgDumper(cfg *config.Config) service.DBDumper {
	return &PgDumper{cfg: &cfg.Database, command: exec.CommandContext}
}

func (d *PgDumper) commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	if d.command != nil {
		return d.command(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...)
}

// Dump executes pg_dump and returns a streaming reader of the output
func (d *PgDumper) Dump(ctx context.Context) (io.ReadCloser, error) {
	args := []string{
		"-h", d.cfg.Host,
		"-p", fmt.Sprintf("%d", d.cfg.Port),
		"-U", d.cfg.User,
		"-d", d.cfg.DBName,
		"--no-owner",
		"--no-acl",
		"--clean",
		"--if-exists",
	}

	cmd := d.commandContext(ctx, "pg_dump", args...)
	if d.cfg.Password != "" {
		cmd.Env = append(cmd.Environ(), "PGPASSWORD="+d.cfg.Password)
	}
	if d.cfg.SSLMode != "" {
		cmd.Env = append(cmd.Environ(), "PGSSLMODE="+d.cfg.SSLMode)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start pg_dump: %w", err)
	}

	// 返回一个 ReadCloser：读 stdout，关闭时等待进程退出
	return &cmdReadCloser{ReadCloser: stdout, cmd: cmd}, nil
}

// Restore executes psql to restore from a streaming reader
func (d *PgDumper) Restore(ctx context.Context, data io.Reader) error {
	args := []string{
		"-h", d.cfg.Host,
		"-p", fmt.Sprintf("%d", d.cfg.Port),
		"-U", d.cfg.User,
		"-d", d.cfg.DBName,
		"--single-transaction",
		"--set", "ON_ERROR_STOP=on",
	}

	cmd := d.commandContext(ctx, "psql", args...)
	if d.cfg.Password != "" {
		cmd.Env = append(cmd.Environ(), "PGPASSWORD="+d.cfg.Password)
	}
	if d.cfg.SSLMode != "" {
		cmd.Env = append(cmd.Environ(), "PGSSLMODE="+d.cfg.SSLMode)
	}

	cmd.Stdin = data

	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql restore failed: %w", err)
	}
	return nil
}

// cmdReadCloser wraps a command stdout pipe and waits for the process on Close
type cmdReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReadCloser) Close() error {
	if c == nil {
		return nil
	}
	// Close the pipe first.
	if c.ReadCloser != nil {
		_ = c.ReadCloser.Close()
	}
	if c.cmd == nil {
		return nil
	}
	// Wait for the producer so a late pg_dump failure cannot be reported as a
	// successful backup after stdout reached EOF.
	if err := c.cmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump exited with error: %w", err)
	}
	return nil
}
