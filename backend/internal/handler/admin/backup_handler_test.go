package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBackupConnectionResultDoesNotExposeInternalError(t *testing.T) {
	result := backupConnectionResult(errors.New("dial https://internal.example: secret-access-key"))
	require.False(t, result.OK)
	require.Equal(t, "连接失败", result.Message)
	require.NotContains(t, result.Message, "internal.example")
	require.NotContains(t, result.Message, "secret-access-key")
}

func TestBackupConnectionResultSuccess(t *testing.T) {
	result := backupConnectionResult(nil)
	require.True(t, result.OK)
	require.Equal(t, "连接成功", result.Message)
}

type backupHandlerSettingRepo struct{ values map[string]string }

func (r *backupHandlerSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (r *backupHandlerSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}
func (r *backupHandlerSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
func (r *backupHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string)
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}
func (r *backupHandlerSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}
func (r *backupHandlerSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *backupHandlerSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

type backupHandlerEncryptor struct{}

func (backupHandlerEncryptor) Encrypt(value string) (string, error) { return value, nil }
func (backupHandlerEncryptor) Decrypt(value string) (string, error) { return value, nil }

type backupHandlerDumper struct{}

func (backupHandlerDumper) Dump(context.Context) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}
func (backupHandlerDumper) Restore(context.Context, io.Reader) error { return errors.New("unused") }

func newBackupHandlerResponseTest(t *testing.T) (*BackupHandler, service.BackupRecord) {
	t.Helper()
	record := service.BackupRecord{
		ID: "opaque-id", Status: "completed", BackupType: "postgres",
		FileName: "private_database_20260715.sql.gz.enc",
		S3Key:    "private-prefix/date/object.enc", ErrorMsg: "internal database diagnostic",
		RestoreError: "internal restore diagnostic", SizeBytes: 123,
	}
	raw, err := json.Marshal([]service.BackupRecord{record})
	require.NoError(t, err)
	repo := &backupHandlerSettingRepo{values: map[string]string{"backup_records": string(raw)}}
	svc := service.NewBackupService(repo, &config.Config{}, backupHandlerEncryptor{}, nil, backupHandlerDumper{}, nil)
	return NewBackupHandler(svc, nil, nil), record
}

func TestBackupRecordEndpointsDoNotExposeInternalTopologyOrDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, record := newBackupHandlerResponseTest(t)

	cases := []struct {
		name   string
		path   string
		invoke func(*gin.Context)
	}{
		{name: "get", path: "/admin/backups/" + record.ID, invoke: func(c *gin.Context) {
			c.Params = []gin.Param{{Key: "id", Value: record.ID}}
			h.GetBackup(c)
		}},
		{name: "list", path: "/admin/backups", invoke: h.ListBackups},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)
			tc.invoke(c)
			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			require.NotContains(t, body, record.FileName)
			require.NotContains(t, body, record.S3Key)
			require.NotContains(t, body, record.ErrorMsg)
			require.NotContains(t, body, record.RestoreError)
			require.Contains(t, body, record.ID)
		})
	}
}
