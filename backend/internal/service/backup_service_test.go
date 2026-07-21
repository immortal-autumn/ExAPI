//go:build unit

package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

// ─── Mocks ───

type mockSettingRepo struct {
	mu             sync.Mutex
	data           map[string]string
	getErrors      map[string]error
	setErrors      map[string]error
	setFailures    map[string]int
	setErrorAt     map[string]map[int]error
	setCalls       map[string]int
	blockGetKey    string
	getEntered     chan struct{}
	releaseGet     chan struct{}
	getEnteredOnce sync.Once
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{
		data:        make(map[string]string),
		getErrors:   make(map[string]error),
		setErrors:   make(map[string]error),
		setFailures: make(map[string]int),
		setErrorAt:  make(map[string]map[int]error),
		setCalls:    make(map[string]int),
	}
}

func (m *mockSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: v}, nil
}

func (m *mockSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if key == m.blockGetKey && m.getEntered != nil && m.releaseGet != nil {
		m.getEnteredOnce.Do(func() { close(m.getEntered) })
		<-m.releaseGet
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.getErrors[key]; err != nil {
		return "", err
	}
	v, ok := m.data[key]
	if !ok {
		return "", nil
	}
	return v, nil
}

func (m *mockSettingRepo) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls[key]++
	if byCall := m.setErrorAt[key]; byCall != nil {
		if err := byCall[m.setCalls[key]]; err != nil {
			return err
		}
	}
	if err := m.setErrors[key]; err != nil && (m.setFailures[key] == 0 || m.setCalls[key] <= m.setFailures[key]) {
		return err
	}
	m.data[key] = value
	return nil
}

func (m *mockSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]string)
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			result[k] = v
		}
	}
	return result, nil
}

func (m *mockSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range settings {
		m.data[k] = v
	}
	return nil
}

func (m *mockSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]string, len(m.data))
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

func (m *mockSettingRepo) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

// plainEncryptor 仅做 base64-like 包装，用于测试
type plainEncryptor struct{}

func (e *plainEncryptor) Encrypt(plaintext string) (string, error) {
	return "ENC:" + plaintext, nil
}

func (e *plainEncryptor) Decrypt(ciphertext string) (string, error) {
	if strings.HasPrefix(ciphertext, "ENC:") {
		return strings.TrimPrefix(ciphertext, "ENC:"), nil
	}
	return ciphertext, fmt.Errorf("not encrypted")
}

type mockDumper struct {
	dumpData []byte
	dumpErr  error
	restored []byte
	restErr  error
}

func (m *mockDumper) Dump(_ context.Context) (io.ReadCloser, error) {
	if m.dumpErr != nil {
		return nil, m.dumpErr
	}
	return io.NopCloser(bytes.NewReader(m.dumpData)), nil
}

func (m *mockDumper) Restore(_ context.Context, data io.Reader) error {
	if m.restErr != nil {
		return m.restErr
	}
	d, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	m.restored = d
	return nil
}

// blockingDumper 可控延迟的 dumper，用于测试异步行为
type blockingDumper struct {
	blockCh   chan struct{}
	restoreCh chan struct{}
	data      []byte
	restErr   error
}

func (d *blockingDumper) Dump(ctx context.Context) (io.ReadCloser, error) {
	select {
	case <-d.blockCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func (d *blockingDumper) Restore(ctx context.Context, data io.Reader) error {
	if d.restErr != nil {
		return d.restErr
	}
	if d.restoreCh != nil {
		select {
		case <-d.restoreCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, _ = io.ReadAll(data)
	return nil
}

type mockObjectStore struct {
	objects        map[string][]byte
	deleteErr      error
	deleteErrByKey map[string]error
	mu             sync.Mutex
}

func newMockObjectStore() *mockObjectStore {
	return &mockObjectStore{objects: make(map[string][]byte), deleteErrByKey: make(map[string]error)}
}

func (m *mockObjectStore) Upload(_ context.Context, key string, body io.Reader, expectedSize int64, _ string) (int64, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	if int64(len(data)) != expectedSize {
		return 0, fmt.Errorf("upload size mismatch: got %d, expected %d", len(data), expectedSize)
	}
	m.mu.Lock()
	m.objects[key] = data
	m.mu.Unlock()
	return int64(len(data)), nil
}

func (m *mockObjectStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockObjectStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if err := m.deleteErrByKey[key]; err != nil {
		return err
	}
	delete(m.objects, key)
	return nil
}

func (m *mockObjectStore) PresignURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://presigned.example.com/" + key, nil
}

func (m *mockObjectStore) HeadBucket(_ context.Context) error {
	return nil
}

func newTestBackupService(repo *mockSettingRepo, dumper DBDumper, store *mockObjectStore) *BackupService {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:   "localhost",
			Port:   5432,
			User:   "test",
			DBName: "testdb",
		},
		// A fixed encryption key is the supported production posture: persisting
		// an S3 secret requires it (#4524).
		Totp: config.TotpConfig{EncryptionKeyConfigured: true},
	}
	factory := func(_ context.Context, _ *BackupS3Config) (BackupObjectStore, error) {
		return store, nil
	}
	backupCipher, err := secretcrypto.NewKeyring("backup-test", map[string][]byte{
		"backup-test": bytes.Repeat([]byte{0x42}, 32),
	})
	if err != nil {
		panic(err)
	}
	return NewBackupService(repo, cfg, &plainEncryptor{}, factory, dumper, backupCipher)
}

// newTestBackupServiceEphemeralKey mirrors a deployment that never set
// TOTP_ENCRYPTION_KEY, so the secret encryption key is auto-generated.
func newTestBackupServiceEphemeralKey(repo *mockSettingRepo) *BackupService {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Host: "localhost", Port: 5432, User: "test", DBName: "testdb"},
		Totp:     config.TotpConfig{EncryptionKeyConfigured: false},
	}
	factory := func(_ context.Context, _ *BackupS3Config) (BackupObjectStore, error) {
		return newMockObjectStore(), nil
	}
	return NewBackupService(repo, cfg, &plainEncryptor{}, factory, &mockDumper{}, nil)
}

func seedS3Config(t *testing.T, repo *mockSettingRepo) {
	t.Helper()
	cfg := BackupS3Config{
		Bucket:          "test-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "ENC:secret123",
		Prefix:          "backups",
	}
	data, _ := json.Marshal(cfg)
	require.NoError(t, repo.Set(context.Background(), settingKeyBackupS3Config, string(data)))
}

// ─── Tests ───

func TestBackupService_LoadS3ConfigRejectsPlaintextCredential(t *testing.T) {
	repo := newMockSettingRepo()
	plaintext := BackupS3Config{
		Bucket:          "legacy-bucket",
		AccessKeyID:     "legacy-id",
		SecretAccessKey: "legacy-plaintext-secret",
	}
	data, err := json.Marshal(plaintext)
	require.NoError(t, err)
	require.NoError(t, repo.Set(context.Background(), settingKeyBackupS3Config, string(data)))
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	cfg, err := svc.loadS3Config(context.Background())
	require.ErrorIs(t, err, ErrBackupS3ConfigCorrupt)
	require.Nil(t, cfg)
}

func TestBackupService_S3ConfigEncryption(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	// 保存配置 -> SecretAccessKey 应被加密
	_, err := svc.UpdateS3Config(context.Background(), BackupS3Config{
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "my-secret",
		Prefix:          "backups",
	})
	require.NoError(t, err)

	// 直接读取数据库中存储的值，应该是加密后的
	raw, _ := repo.GetValue(context.Background(), settingKeyBackupS3Config)
	var stored BackupS3Config
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Equal(t, "ENC:my-secret", stored.SecretAccessKey)

	// 通过 GetS3Config 获取应该脱敏
	cfg, err := svc.GetS3Config(context.Background())
	require.NoError(t, err)
	require.Empty(t, cfg.SecretAccessKey)
	require.Equal(t, "my-bucket", cfg.Bucket)

	// loadS3Config 内部应解密
	internal, err := svc.loadS3Config(context.Background())
	require.NoError(t, err)
	require.Equal(t, "my-secret", internal.SecretAccessKey)
}

func TestBackupService_S3ConfigKeepExistingSecret(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	// 先保存一个有 secret 的配置
	_, err := svc.UpdateS3Config(context.Background(), BackupS3Config{
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "original-secret",
	})
	require.NoError(t, err)

	// 再更新时不提供 secret，应保留原值
	_, err = svc.UpdateS3Config(context.Background(), BackupS3Config{
		Bucket:      "my-bucket",
		AccessKeyID: "AKID-NEW",
	})
	require.NoError(t, err)

	raw, err := repo.GetValue(context.Background(), settingKeyBackupS3Config)
	require.NoError(t, err)
	var stored BackupS3Config
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Equal(t, "ENC:original-secret", stored.SecretAccessKey)

	internal, err := svc.loadS3Config(context.Background())
	require.NoError(t, err)
	require.Equal(t, "original-secret", internal.SecretAccessKey)
	require.Equal(t, "AKID-NEW", internal.AccessKeyID)
}

func TestBackupService_SaveRecordDoesNotDropRestorableMetadataAtHistoryLimit(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	const historyLimitRegressionCount = 101
	for i := 0; i < historyLimitRegressionCount; i++ {
		record := &BackupRecord{
			ID:        fmt.Sprintf("backup-%03d", i),
			Status:    "completed",
			S3Key:     fmt.Sprintf("backups/backup-%03d", i),
			StartedAt: time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
		require.NoError(t, svc.saveRecord(context.Background(), record))
	}

	records, err := svc.loadRecords(context.Background())
	require.NoError(t, err)
	require.Len(t, records, historyLimitRegressionCount, "metadata must not be truncated while referenced backup objects still exist")
	require.Equal(t, "backup-000", records[0].ID)
}

func TestBackupService_UpdateS3Config_RejectsEphemeralKey(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupServiceEphemeralKey(repo)

	// 提供新 secret 但密钥为自动生成 -> 必须拒绝，避免重启后无法解密（#4524）。
	_, err := svc.UpdateS3Config(context.Background(), BackupS3Config{
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "my-secret",
		Prefix:          "backups",
	})
	require.ErrorIs(t, err, ErrSecretEncryptionKeyNotConfigured)

	// 不应写入任何配置。
	raw, _ := repo.GetValue(context.Background(), settingKeyBackupS3Config)
	require.Empty(t, raw)
}

func TestBackupService_UpdateS3Config_NoSecretAllowedWithEphemeralKey(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupServiceEphemeralKey(repo)

	// 不含 secret 的更新（如只改 bucket）不触碰加密路径，应放行。
	_, err := svc.UpdateS3Config(context.Background(), BackupS3Config{
		Bucket:      "my-bucket",
		AccessKeyID: "AKID",
	})
	require.NoError(t, err)
}

func TestBackupService_EncryptionKeyConfigured(t *testing.T) {
	repo := newMockSettingRepo()
	require.True(t, newTestBackupService(repo, &mockDumper{}, newMockObjectStore()).EncryptionKeyConfigured())
	require.False(t, newTestBackupServiceEphemeralKey(repo).EncryptionKeyConfigured())
}

func TestBackupService_SaveRecordConcurrency(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	var wg sync.WaitGroup
	n := 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			record := &BackupRecord{
				ID:        fmt.Sprintf("rec-%d", idx),
				Status:    "completed",
				StartedAt: time.Now().Format(time.RFC3339),
			}
			_ = svc.saveRecord(context.Background(), record)
		}(i)
	}
	wg.Wait()

	records, err := svc.loadRecords(context.Background())
	require.NoError(t, err)
	require.Len(t, records, n)
}

func TestBackupService_LoadRecords_Empty(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	records, err := svc.loadRecords(context.Background())
	require.NoError(t, err)
	require.Nil(t, records) // 无数据时返回 nil
}

func TestBackupService_LoadRecords_MissingSetting(t *testing.T) {
	repo := newMockSettingRepo()
	repo.getErrors[settingKeyBackupRecords] = ErrSettingNotFound
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	records, err := svc.loadRecords(context.Background())
	require.NoError(t, err)
	require.Nil(t, records)
}

func TestS3ConnectionPropagatesStoredCredentialReadError(t *testing.T) {
	repo := newMockSettingRepo()
	repo.getErrors[settingKeyBackupS3Config] = errors.New("repository unavailable")
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	err := svc.TestS3Connection(context.Background(), BackupS3Config{Bucket: "bucket", AccessKeyID: "id"})
	require.ErrorContains(t, err, "repository unavailable")
}

func TestGetS3ConfigTreatsMissingSettingAsUnconfigured(t *testing.T) {
	repo := newMockSettingRepo()
	repo.getErrors[settingKeyBackupS3Config] = ErrSettingNotFound
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	cfg, err := svc.GetS3Config(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.Bucket)
}

func TestS3ConnectionWithMissingStoredConfigReturnsIncompleteConfig(t *testing.T) {
	repo := newMockSettingRepo()
	repo.getErrors[settingKeyBackupS3Config] = ErrSettingNotFound
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	err := svc.TestS3Connection(context.Background(), BackupS3Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "S3 配置不完整")
	require.NotContains(t, err.Error(), "setting not found")
}

func TestUpdateS3ConfigDoesNotOverwriteCredentialAfterReadError(t *testing.T) {
	repo := newMockSettingRepo()
	repo.getErrors[settingKeyBackupS3Config] = errors.New("repository unavailable")
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	_, err := svc.UpdateS3Config(context.Background(), BackupS3Config{Bucket: "new", AccessKeyID: "id"})
	require.ErrorContains(t, err, "repository unavailable")
	require.Zero(t, repo.setCalls[settingKeyBackupS3Config])
}

func TestGetOrCreateStoreRebuildsWhenConfigChanges(t *testing.T) {
	repo := newMockSettingRepo()
	calls := 0
	factory := func(_ context.Context, _ *BackupS3Config) (BackupObjectStore, error) {
		calls++
		return newMockObjectStore(), nil
	}
	cipher, err := secretcrypto.NewKeyring("backup-test", map[string][]byte{"backup-test": bytes.Repeat([]byte{7}, 32)})
	require.NoError(t, err)
	svc := NewBackupService(repo, &config.Config{Database: config.DatabaseConfig{DBName: "testdb"}}, &plainEncryptor{}, factory, &mockDumper{}, cipher)
	cfg1 := &BackupS3Config{Endpoint: "https://s3.example.com", Region: "r1", Bucket: "one", AccessKeyID: "a", SecretAccessKey: "s"}
	cfg2 := &BackupS3Config{Endpoint: "https://s3.example.com", Region: "r1", Bucket: "two", AccessKeyID: "a", SecretAccessKey: "s"}

	_, err = svc.getOrCreateStore(context.Background(), cfg1)
	require.NoError(t, err)
	_, err = svc.getOrCreateStore(context.Background(), cfg2)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestBuildS3KeyIncludesUniqueBackupID(t *testing.T) {
	svc := &BackupService{}
	cfg := &BackupS3Config{Prefix: "backups"}
	key1 := svc.buildS3Key(cfg, "db.sql.gz.enc", "id-one")
	key2 := svc.buildS3Key(cfg, "db.sql.gz.enc", "id-two")
	require.NotEqual(t, key1, key2)
}

func TestBackupService_LoadRecords_Corrupted(t *testing.T) {
	repo := newMockSettingRepo()
	_ = repo.Set(context.Background(), settingKeyBackupRecords, "not valid json{{{")
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	records, err := svc.loadRecords(context.Background())
	require.Error(t, err) // 损坏数据应返回错误
	require.Nil(t, records)
}

func TestBackupService_SaveRecordDoesNotOverwriteCorruptMetadata(t *testing.T) {
	repo := newMockSettingRepo()
	const corrupt = "not valid json{{{"
	require.NoError(t, repo.Set(context.Background(), settingKeyBackupRecords, corrupt))
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	err := svc.saveRecord(context.Background(), &BackupRecord{ID: "new", Status: "completed"})
	require.ErrorIs(t, err, ErrBackupRecordsCorrupt)

	stored, loadErr := repo.GetValue(context.Background(), settingKeyBackupRecords)
	require.NoError(t, loadErr)
	require.Equal(t, corrupt, stored)
}

func TestBackupService_SaveRecordDoesNotWriteAfterMetadataReadError(t *testing.T) {
	repo := newMockSettingRepo()
	readErr := errors.New("transient repository read failure")
	repo.getErrors[settingKeyBackupRecords] = readErr
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	err := svc.saveRecord(context.Background(), &BackupRecord{ID: "new", Status: "completed"})
	require.ErrorIs(t, err, readErr)
	require.Zero(t, repo.setCalls[settingKeyBackupRecords])
}

func TestBackupRecordForAPI_RedactsInternalFailureDetails(t *testing.T) {
	record := BackupRecord{
		ID:            "backup-1",
		Status:        "failed",
		FileName:      "internal_database_20260715.sql.gz.enc",
		S3Key:         "private-prefix/2026/07/15/object.enc",
		ErrorMsg:      "database password=[REDACTED] host=internal-db",
		RestoreStatus: "failed",
		RestoreError:  "psql echoed secret SQL",
	}

	redacted := BackupRecordForAPI(record)
	require.Empty(t, redacted.ErrorMsg)
	require.Empty(t, redacted.RestoreError)
	require.Empty(t, redacted.FileName)
	require.Empty(t, redacted.S3Key)
	require.Equal(t, record.ID, redacted.ID)
	require.Equal(t, record.Status, redacted.Status)
}

func TestBackupService_CreateBackup_Streaming(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumpContent := "-- PostgreSQL dump\nCREATE TABLE test (id int);\n"
	dumper := &mockDumper{dumpData: []byte(dumpContent)}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	require.Equal(t, "completed", record.Status)
	require.Equal(t, backupFormatEncryptedV1, record.Format)
	require.Greater(t, record.SizeBytes, int64(0))
	require.NotEmpty(t, record.S3Key)

	// 验证 S3 上确实有文件
	store.mu.Lock()
	require.Len(t, store.objects, 1)
	stored := append([]byte(nil), store.objects[record.S3Key]...)
	store.mu.Unlock()
	require.True(t, bytes.HasPrefix(stored, []byte("S2BKENC1")), "stored backup must use the encrypted stream format")
	require.NotContains(t, string(stored), "CREATE TABLE test", "stored backup must not expose dump plaintext")
}

func TestCreateBackupReturnsTerminalPersistenceError(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	persistErr := errors.New("terminal metadata persistence failed")
	repo.setErrors[settingKeyBackupRecords] = persistErr
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, persistErr)
	require.Equal(t, "running", record.Status)
	store.mu.Lock()
	defer store.mu.Unlock()
	require.Empty(t, store.objects, "backup upload must not start until its initial metadata record is durable")
}

func TestCompletionPersistenceAndCompensationFailureLeavesRetryableTombstone(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "asynchronous"}[async], func(t *testing.T) {
			repo := newMockSettingRepo()
			seedS3Config(t, repo)
			store := newMockObjectStore()
			store.deleteErr = errors.New("compensation delete failed")
			svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)
			if async {
				repo.setErrorAt[settingKeyBackupRecords] = map[int]error{4: errors.New("completion persistence failed")}
				record, err := svc.StartBackup(context.Background(), "manual", 14)
				require.NoError(t, err)
				svc.wg.Wait()
				stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
				require.NoError(t, loadErr)
				require.Equal(t, "deleting", stored.Status)
			} else {
				repo.setErrorAt[settingKeyBackupRecords] = map[int]error{3: errors.New("completion persistence failed")}
				record, err := svc.CreateBackup(context.Background(), "manual", 14)
				require.Error(t, err)
				stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
				require.NoError(t, loadErr)
				require.Equal(t, "deleting", stored.Status)
			}
			store.deleteErr = nil
			require.NoError(t, svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{}))
		})
	}
}

func TestSynchronousBackupPersistsUploadingBeforeExternalUpload(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	svc := newTestBackupService(repo, dumper, store)
	done := make(chan error, 1)
	go func() {
		_, err := svc.CreateBackup(context.Background(), "scheduled", 14)
		done <- err
	}()

	require.Eventually(t, func() bool {
		records, err := svc.loadRecords(context.Background())
		return err == nil && len(records) == 1 && records[0].Progress == "uploading"
	}, time.Second, 10*time.Millisecond)
	close(dumper.blockCh)
	require.NoError(t, <-done)
}

func TestSynchronousCompletionPersistenceFailureCompensatesUploadedObject(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	persistErr := errors.New("completed metadata persistence failed")
	// Initial running persists; completed transition fails once; compensated
	// failed state then persists.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{3: persistErr}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, persistErr)
	store.mu.Lock()
	require.Empty(t, store.objects, "uploaded object must be removed when completion metadata cannot become durable")
	store.mu.Unlock()
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	require.Equal(t, "failed", stored.Status)
	require.Empty(t, stored.Progress)
	require.NoError(t, svc.DeleteBackup(context.Background(), record.ID))
	_, loadErr = svc.GetBackupRecord(context.Background(), record.ID)
	require.ErrorIs(t, loadErr, ErrBackupNotFound)
}

func TestSynchronousBackupReturnsFailureMetadataPersistenceError(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	persistErr := errors.New("failure metadata persistence failed")
	// Initial running write succeeds; the failed terminal write does not.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{3: persistErr}
	svc := newTestBackupService(repo, &mockDumper{dumpErr: errors.New("pg_dump failed")}, newMockObjectStore())

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, persistErr)
	require.Equal(t, "failed", record.Status)
}

func TestBackupService_CreateBackup_DumpFailure(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumper := &mockDumper{dumpErr: fmt.Errorf("pg_dump failed")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.Error(t, err)
	require.Equal(t, "failed", record.Status)
	require.Contains(t, record.ErrorMsg, "pg_dump")
}

func TestBackupService_CreateBackup_NoS3Config(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	_, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrBackupS3NotConfigured)
}

func TestBackupService_CreateBackup_ConcurrentBlocked(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	// 使用一个慢速 dumper 来模拟正在进行的备份
	dumper := &mockDumper{dumpData: []byte("data")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	// 手动设置 backingUp 标志
	svc.opMu.Lock()
	svc.backingUp = true
	svc.opMu.Unlock()

	_, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrBackupInProgress)
}

func TestCreateBackupAdmissionIsAtomicWithStop(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	svc := newTestBackupService(repo, dumper, newMockObjectStore())

	backupDone := make(chan error, 1)
	go func() {
		_, err := svc.CreateBackup(context.Background(), "manual", 14)
		backupDone <- err
	}()
	time.Sleep(50 * time.Millisecond)

	stopDone := make(chan struct{})
	go func() { svc.Stop(); close(stopDone) }()
	select {
	case <-stopDone:
		close(dumper.blockCh)
		t.Fatal("Stop returned before synchronous backup completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(dumper.blockCh)
	require.NoError(t, <-backupDone)
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after synchronous backup completed")
	}
}

func TestBackupService_RestoreBackup_Streaming(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumpContent := "-- PostgreSQL dump\nCREATE TABLE test (id int);\n"
	dumper := &mockDumper{dumpData: []byte(dumpContent)}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	// 先创建一个备份
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	// 恢复
	err = svc.RestoreBackup(context.Background(), record.ID)
	require.NoError(t, err)

	// 验证 psql 收到的数据是否与原始 dump 内容一致
	require.Equal(t, dumpContent, string(dumper.restored))
}

func TestRestoreBackupAdmissionIsAtomicWithStop(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	seedDumper := &mockDumper{dumpData: []byte("SELECT 1;\n")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, seedDumper, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	restoreGate := make(chan struct{})
	svc.dumper = &blockingDumper{restoreCh: restoreGate}
	restoreDone := make(chan error, 1)
	go func() { restoreDone <- svc.RestoreBackup(context.Background(), record.ID) }()
	time.Sleep(50 * time.Millisecond)

	stopDone := make(chan struct{})
	go func() { svc.Stop(); close(stopDone) }()
	select {
	case <-stopDone:
		close(restoreGate)
		t.Fatal("Stop returned before synchronous restore completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(restoreGate)
	require.NoError(t, <-restoreDone)
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after synchronous restore completed")
	}
}

func TestBackupService_RestoreBackup_RejectsTamperingBeforeDumper(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "true")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{dumpData: []byte("SELECT 'secret';\n")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	store.mu.Lock()
	stored := store.objects[record.S3Key]
	require.NotEmpty(t, stored)
	stored[len(stored)-1] ^= 0x80
	store.objects[record.S3Key] = stored
	store.mu.Unlock()
	dumper.restored = nil

	err = svc.RestoreBackup(context.Background(), record.ID)
	require.Error(t, err)
	require.Empty(t, dumper.restored, "database restore must not start before backup authentication succeeds")
}

func TestBackupService_RestoreBackup_NotCompleted(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	// 手动插入一条 failed 记录
	_ = svc.saveRecord(context.Background(), &BackupRecord{
		ID:     "fail-1",
		Status: "failed",
	})

	err := svc.RestoreBackup(context.Background(), "fail-1")
	require.Error(t, err)
}

func TestCleanupOldBackupsRetainCountCountsOnlyCompletedBackups(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{}, store)
	now := time.Now()
	failed := BackupRecord{ID: "failed-newest", Status: "failed", StartedAt: now.Format(time.RFC3339)}
	newestCompleted := BackupRecord{ID: "completed-newest", Status: "completed", S3Key: "backups/completed-newest", StartedAt: now.Add(-time.Hour).Format(time.RFC3339)}
	olderCompleted := BackupRecord{ID: "completed-older", Status: "completed", S3Key: "backups/completed-older", StartedAt: now.Add(-2 * time.Hour).Format(time.RFC3339)}
	for _, record := range []BackupRecord{failed, newestCompleted, olderCompleted} {
		require.NoError(t, svc.saveRecord(context.Background(), &record))
		if record.S3Key != "" {
			store.objects[record.S3Key] = []byte("encrypted")
		}
	}

	require.NoError(t, svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainCount: 1}))
	_, err := svc.GetBackupRecord(context.Background(), newestCompleted.ID)
	require.NoError(t, err, "newest completed backup must be retained even when a newer failed record exists")
	_, err = svc.GetBackupRecord(context.Background(), olderCompleted.ID)
	require.ErrorIs(t, err, ErrBackupNotFound)
	_, err = svc.GetBackupRecord(context.Background(), failed.ID)
	require.NoError(t, err)
}

func TestCleanupOldBackupsFailsClosedOnMalformedTimestamp(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{}, store)
	record := BackupRecord{ID: "malformed-time", Status: "completed", S3Key: "backups/malformed", StartedAt: "not-rfc3339"}
	require.NoError(t, svc.saveRecord(context.Background(), &record))
	store.objects[record.S3Key] = []byte("encrypted")

	err := svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainDays: 1})
	require.Error(t, err)
	store.mu.Lock()
	require.Contains(t, store.objects, record.S3Key)
	store.mu.Unlock()
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	require.Equal(t, "completed", stored.Status)
}

func TestCleanupOldBackupsRetriesAfterObjectDeleteMetadataFailure(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{}, store)
	record := BackupRecord{ID: "old", Status: "completed", S3Key: "backups/old", StartedAt: time.Now().Add(-72 * time.Hour).Format(time.RFC3339)}
	require.NoError(t, svc.saveRecord(context.Background(), &record))
	store.objects[record.S3Key] = []byte("encrypted")
	currentCalls := repo.setCalls[settingKeyBackupRecords]
	// Persisting deleting succeeds; removing metadata after object deletion fails.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{currentCalls + 2: errors.New("metadata removal failed")}

	err := svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainDays: 1})
	require.ErrorContains(t, err, "metadata removal failed")
	store.mu.Lock()
	require.NotContains(t, store.objects, record.S3Key)
	store.mu.Unlock()
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	require.Equal(t, "deleting", stored.Status, "durable tombstone must survive metadata removal failure")

	repo.setErrorAt[settingKeyBackupRecords] = nil
	require.NoError(t, svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainDays: 1}))
	_, loadErr = svc.GetBackupRecord(context.Background(), record.ID)
	require.ErrorIs(t, loadErr, ErrBackupNotFound)
}

func TestCleanupOldBackupsPersistsEarlierDeletesWhenLaterDeletionFails(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{}, store)

	oldest := BackupRecord{ID: "oldest", Status: "completed", S3Key: "backups/oldest", StartedAt: time.Now().Add(-72 * time.Hour).Format(time.RFC3339)}
	newer := BackupRecord{ID: "newer", Status: "completed", S3Key: "backups/newer", StartedAt: time.Now().Add(-48 * time.Hour).Format(time.RFC3339)}
	require.NoError(t, svc.saveRecord(context.Background(), &oldest))
	require.NoError(t, svc.saveRecord(context.Background(), &newer))
	store.objects[oldest.S3Key] = []byte("oldest")
	store.objects[newer.S3Key] = []byte("newer")
	store.deleteErrByKey[oldest.S3Key] = errors.New("later deletion failed")

	err := svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainDays: 1})
	require.ErrorContains(t, err, "later deletion failed")

	records, loadErr := svc.loadRecords(context.Background())
	require.NoError(t, loadErr)
	require.Len(t, records, 1)
	require.Equal(t, oldest.ID, records[0].ID, "metadata for a successfully deleted object must not survive a later deletion failure")
	store.mu.Lock()
	defer store.mu.Unlock()
	require.Contains(t, store.objects, oldest.S3Key)
	require.NotContains(t, store.objects, newer.S3Key)
}

func TestCleanupOldBackupsRetainsMetadataWhenDeletionFails(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("data")}, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	store.deleteErr = errors.New("retention delete failed")

	err = svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainCount: 0, RetainDays: 1})
	// Force expiration deterministically by rewriting the timestamp.
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	stored.StartedAt = time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	require.NoError(t, svc.saveRecord(context.Background(), stored))
	err = svc.cleanupOldBackups(context.Background(), &BackupScheduleConfig{RetainDays: 1})
	require.ErrorContains(t, err, "retention delete failed")
	_, loadErr = svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
}

func TestDeleteBackupRetriesAfterObjectDeleteMetadataFailure(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("data")}, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	currentCalls := repo.setCalls[settingKeyBackupRecords]
	// Persist deletion intent; fail metadata removal after object deletion.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{currentCalls + 2: errors.New("manual metadata removal failed")}

	err = svc.DeleteBackup(context.Background(), record.ID)
	require.ErrorContains(t, err, "manual metadata removal failed")
	store.mu.Lock()
	require.NotContains(t, store.objects, record.S3Key)
	store.mu.Unlock()
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	require.Equal(t, "deleting", stored.Status)

	repo.setErrorAt[settingKeyBackupRecords] = nil
	require.NoError(t, svc.DeleteBackup(context.Background(), record.ID))
	_, loadErr = svc.GetBackupRecord(context.Background(), record.ID)
	require.ErrorIs(t, loadErr, ErrBackupNotFound)
}

func TestDeleteBackupRetainsMetadataWhenObjectDeletionFails(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("data")}, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	deleteErr := errors.New("s3 deletion failed")
	store.deleteErr = deleteErr
	err = svc.DeleteBackup(context.Background(), record.ID)
	require.ErrorIs(t, err, deleteErr)
	stored, loadErr := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, loadErr)
	require.Equal(t, record.ID, stored.ID)
}

func TestBackupService_DeleteBackup(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumpContent := "data"
	dumper := &mockDumper{dumpData: []byte(dumpContent)}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	// S3 中应有文件
	store.mu.Lock()
	require.Len(t, store.objects, 1)
	store.mu.Unlock()

	// 删除
	err = svc.DeleteBackup(context.Background(), record.ID)
	require.NoError(t, err)

	// S3 中文件应被删除
	store.mu.Lock()
	require.Len(t, store.objects, 0)
	store.mu.Unlock()

	// 记录应不存在
	_, err = svc.GetBackupRecord(context.Background(), record.ID)
	require.ErrorIs(t, err, ErrBackupNotFound)
}

func TestBackupService_GetDownloadURL(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumper := &mockDumper{dumpData: []byte("data")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	url, err := svc.GetBackupDownloadURL(context.Background(), record.ID)
	require.NoError(t, err)
	require.Contains(t, url, "https://presigned.example.com/")
}

func TestBackupService_ListBackups_Sorted(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	now := time.Now()
	for i := 0; i < 3; i++ {
		_ = svc.saveRecord(context.Background(), &BackupRecord{
			ID:        fmt.Sprintf("rec-%d", i),
			Status:    "completed",
			StartedAt: now.Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
		})
	}

	records, err := svc.ListBackups(context.Background())
	require.NoError(t, err)
	require.Len(t, records, 3)
	// 最新在前
	require.Equal(t, "rec-2", records[0].ID)
	require.Equal(t, "rec-0", records[2].ID)
}

func TestBackupService_TestS3Connection(t *testing.T) {
	repo := newMockSettingRepo()
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{}, store)

	err := svc.TestS3Connection(context.Background(), BackupS3Config{
		Bucket:          "test",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	})
	require.NoError(t, err)
}

func TestBackupService_TestS3Connection_Incomplete(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	err := svc.TestS3Connection(context.Background(), BackupS3Config{
		Bucket: "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "S3 配置不完整")
}

func TestBackupServiceRejectsNegativeRetentionValues(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	_, err := svc.UpdateSchedule(context.Background(), BackupScheduleConfig{RetainDays: -1})
	require.Error(t, err)
	require.Zero(t, repo.setCalls[settingKeyBackupSchedule])

	_, err = svc.UpdateSchedule(context.Background(), BackupScheduleConfig{RetainCount: -1})
	require.Error(t, err)
	require.Zero(t, repo.setCalls[settingKeyBackupSchedule])

	_, err = svc.CreateBackup(context.Background(), "manual", -1)
	require.Error(t, err)
	_, err = svc.StartBackup(context.Background(), "manual", -1)
	require.Error(t, err)
}

func TestBackupService_GetScheduleFailsClosedOnRepositoryAndCorruptData(t *testing.T) {
	t.Run("repository error", func(t *testing.T) {
		repo := newMockSettingRepo()
		readErr := errors.New("repository unavailable")
		repo.getErrors[settingKeyBackupSchedule] = readErr
		svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

		cfg, err := svc.GetSchedule(context.Background())
		require.ErrorIs(t, err, readErr)
		require.Nil(t, cfg)
	})

	t.Run("corrupt json", func(t *testing.T) {
		repo := newMockSettingRepo()
		require.NoError(t, repo.Set(context.Background(), settingKeyBackupSchedule, "{not-json"))
		svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

		cfg, err := svc.GetSchedule(context.Background())
		require.ErrorIs(t, err, ErrBackupScheduleCorrupt)
		require.Nil(t, cfg)
	})
}

func TestBackupService_ScheduleDoesNotPersistWhenSchedulerUnavailable(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())
	svc.cronSched = nil

	_, err := svc.UpdateSchedule(context.Background(), BackupScheduleConfig{
		Enabled:    true,
		CronExpr:   "0 2 * * *",
		RetainDays: 14,
	})
	require.Error(t, err)
	require.Zero(t, repo.setCalls[settingKeyBackupSchedule], "enabled schedule must not persist when it cannot be installed")
}

func TestBackupService_Schedule_CronValidation(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())
	svc.cronSched = nil // 未初始化 cron

	// 启用但 cron 为空
	_, err := svc.UpdateSchedule(context.Background(), BackupScheduleConfig{
		Enabled:  true,
		CronExpr: "",
	})
	require.Error(t, err)

	// 无效的 cron 表达式
	_, err = svc.UpdateSchedule(context.Background(), BackupScheduleConfig{
		Enabled:  true,
		CronExpr: "invalid",
	})
	require.Error(t, err)
}

func TestBackupService_LoadS3Config_Corrupted(t *testing.T) {
	repo := newMockSettingRepo()
	_ = repo.Set(context.Background(), settingKeyBackupS3Config, "not json!!!!")
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	cfg, err := svc.loadS3Config(context.Background())
	require.Error(t, err)
	require.Nil(t, cfg)
}

// ─── Async Backup Tests ───

func TestStartBackup_ReturnsImmediately(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	record, err := svc.StartBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	require.Equal(t, "running", record.Status)
	require.NotEmpty(t, record.ID)

	// 释放 dumper 让后台完成
	close(dumper.blockCh)
	svc.wg.Wait()

	// 验证最终状态
	final, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", final.Status)
	require.Greater(t, final.SizeBytes, int64(0))
}

func TestBackupAndRestoreAreMutuallyExclusive(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	svc.opMu.Lock()
	svc.backingUp = true
	svc.opMu.Unlock()
	_, err := svc.StartRestore(context.Background(), "missing")
	require.ErrorIs(t, err, ErrBackupInProgress)

	svc.opMu.Lock()
	svc.backingUp = false
	svc.restoring = true
	svc.opMu.Unlock()
	_, err = svc.StartBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrRestoreInProgress)
}

func TestStartBackup_ConcurrentBlocked(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	// 第一次启动
	_, err := svc.StartBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	// 第二次应被阻塞
	_, err = svc.StartBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrBackupInProgress)

	close(dumper.blockCh)
	svc.wg.Wait()
}

func TestBackupServiceStartIsSingleAndCannotRestartAfterStop(t *testing.T) {
	svc := newTestBackupService(newMockSettingRepo(), &mockDumper{}, newMockObjectStore())
	svc.Start()
	first := svc.cronSched
	svc.Start()
	require.Same(t, first, svc.cronSched)
	svc.Stop()
	svc.Start()
	require.Same(t, first, svc.cronSched)
}

func TestStartBackup_ShuttingDown(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("data")}, newMockObjectStore())

	svc.shuttingDown.Store(true)

	_, err := svc.StartBackup(context.Background(), "manual", 14)
	require.Error(t, err)
	require.Contains(t, err.Error(), "shutting down")
}

func TestStartBackupAdmissionIsAtomicWithStop(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	repo.blockGetKey = settingKeyBackupS3Config
	repo.getEntered = make(chan struct{})
	repo.releaseGet = make(chan struct{})

	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	svc := newTestBackupService(repo, dumper, newMockObjectStore())

	startDone := make(chan error, 1)
	go func() {
		_, err := svc.StartBackup(context.Background(), "manual", 14)
		startDone <- err
	}()
	<-repo.getEntered

	stopDone := make(chan struct{})
	go func() {
		svc.Stop()
		close(stopDone)
	}()

	stopReturnedEarly := false
	select {
	case <-stopDone:
		stopReturnedEarly = true
	case <-time.After(100 * time.Millisecond):
	}

	close(repo.releaseGet)
	startErr := <-startDone
	if stopReturnedEarly {
		close(dumper.blockCh)
		t.Fatal("Stop returned while an accepted backup was not yet registered")
	}
	require.NoError(t, startErr)

	// Once admission is registered, shutdown cancellation must release the
	// cooperative operation rather than waiting for its normal work signal.
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		close(dumper.blockCh)
		t.Fatal("Stop did not cancel and join the admitted backup")
	}
	close(dumper.blockCh)
}

func TestRecoverStaleUploadedBackupCreatesCleanupTombstone(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())
	record := BackupRecord{ID: "stale-upload", Status: "running", S3Key: "backups/stale-upload", Progress: "uploading"}
	require.NoError(t, svc.saveRecord(context.Background(), &record))

	require.NoError(t, svc.recoverStaleRecords())
	stored, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "deleting", stored.Status, "a crash after upload must leave retryable object cleanup, not an inert failed record")
	require.Empty(t, stored.Progress)
}

func TestRecoverStaleRecordsReturnsPersistenceFailure(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())
	require.NoError(t, svc.saveRecord(context.Background(), &BackupRecord{ID: "stale", Status: "running"}))
	persistErr := errors.New("stale recovery persistence failed")
	repo.setErrors[settingKeyBackupRecords] = persistErr

	err := svc.recoverStaleRecords()
	require.ErrorIs(t, err, persistErr)
}

func TestRecoverStaleRecords(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())

	// 模拟一条孤立的 running 记录
	_ = svc.saveRecord(context.Background(), &BackupRecord{
		ID:        "stale-1",
		Status:    "running",
		StartedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	})
	// 模拟一条孤立的恢复中记录
	_ = svc.saveRecord(context.Background(), &BackupRecord{
		ID:            "stale-2",
		Status:        "completed",
		RestoreStatus: "running",
		StartedAt:     time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	})

	require.NoError(t, svc.recoverStaleRecords())

	r1, _ := svc.GetBackupRecord(context.Background(), "stale-1")
	require.Equal(t, "failed", r1.Status)
	require.Contains(t, r1.ErrorMsg, "server restart")

	r2, _ := svc.GetBackupRecord(context.Background(), "stale-2")
	require.Equal(t, "failed", r2.RestoreStatus)
	require.Contains(t, r2.RestoreError, "server restart")
}

func TestStopContextHonorsDeadlineForUncooperativeOperation(t *testing.T) {
	svc := newTestBackupService(newMockSettingRepo(), &mockDumper{}, newMockObjectStore())
	svc.wg.Add(1) // Simulate an operation that does not respond to cancellation.
	defer svc.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := svc.StopContext(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), 500*time.Millisecond)
	require.True(t, svc.shuttingDown.Load())
}

func TestGracefulShutdown(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumper := &blockingDumper{blockCh: make(chan struct{}), data: []byte("data")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	_, err := svc.StartBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	// Stop cancels cooperative background work and joins it.
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(dumper.blockCh)
		t.Fatal("Stop did not cancel and join backup")
	}
	close(dumper.blockCh)
}

func TestAsyncBackupAbortsBeforeUploadWhenProgressPersistenceFails(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	persistErr := errors.New("dumping progress persistence failed")
	// Initial running succeeds; the first background progress update fails.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{2: persistErr}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)

	record, err := svc.StartBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	svc.wg.Wait()

	store.mu.Lock()
	require.Empty(t, store.objects, "upload must not proceed after state persistence fails")
	store.mu.Unlock()
	stored, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", stored.Status)
	require.Contains(t, stored.ErrorMsg, "persist dumping progress")
}

func TestAsyncBackupCompensatesObjectWhenCompletionMetadataFails(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	persistErr := errors.New("terminal metadata persistence failed")
	// Initial running, dumping, and uploading writes succeed; completed fails;
	// compensated failed state succeeds.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{4: persistErr}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)

	record, err := svc.StartBackup(context.Background(), "manual", 14)
	require.NoError(t, err)
	svc.wg.Wait()

	store.mu.Lock()
	require.Empty(t, store.objects, "async uploaded object must be compensated when completion metadata fails")
	store.mu.Unlock()
	stored, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", stored.Status)
	require.Contains(t, stored.ErrorMsg, "rolled back")
	require.NoError(t, svc.DeleteBackup(context.Background(), record.ID))
}

func TestAsyncRestorePersistsTerminalMetadataFailureState(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	persistErr := errors.New("restore terminal metadata persistence failed")
	currentCalls := repo.setCalls[settingKeyBackupRecords]
	// Running status succeeds, completed status fails, failure-state retry succeeds.
	repo.setErrorAt[settingKeyBackupRecords] = map[int]error{currentCalls + 2: persistErr}
	_, err = svc.StartRestore(context.Background(), record.ID)
	require.NoError(t, err)
	svc.wg.Wait()

	stored, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", stored.RestoreStatus)
	require.Contains(t, stored.RestoreError, "persist completed restore metadata")
}

func TestStartRestoreReturnsInitialPersistenceError(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	store := newMockObjectStore()
	svc := newTestBackupService(repo, &mockDumper{dumpData: []byte("SELECT 1;\n")}, store)
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	persistErr := errors.New("restore status persistence failed")
	repo.setErrors[settingKeyBackupRecords] = persistErr
	_, err = svc.StartRestore(context.Background(), record.ID)
	require.ErrorIs(t, err, persistErr)

	svc.opMu.Lock()
	restoring := svc.restoring
	svc.opMu.Unlock()
	require.False(t, restoring)
}

func TestStartRestore_Async(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)

	dumpContent := "-- PostgreSQL dump\nCREATE TABLE test (id int);\n"
	dumper := &mockDumper{dumpData: []byte(dumpContent)}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)

	// 先创建一个备份（同步方式）
	record, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.NoError(t, err)

	// 异步恢复
	restored, err := svc.StartRestore(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "running", restored.RestoreStatus)

	svc.wg.Wait()

	// 验证最终状态
	final, err := svc.GetBackupRecord(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", final.RestoreStatus)
}

func TestBackupService_BackupBoundariesFailBeforeSideEffectsWithoutKeyring(t *testing.T) {
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{dumpErr: errors.New("dumper must not be called")}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	svc.backupCipher = nil

	_, err := svc.CreateBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrBackupEncryptionNotConfigured)
	_, err = svc.StartBackup(context.Background(), "manual", 14)
	require.ErrorIs(t, err, ErrBackupEncryptionNotConfigured)
	records, loadErr := svc.loadRecords(context.Background())
	require.NoError(t, loadErr)
	require.Empty(t, records)
	require.Empty(t, store.objects)
}

func TestBackupService_RestoreBoundariesLookupRecordBeforeRequiringKeyring(t *testing.T) {
	repo := newMockSettingRepo()
	svc := newTestBackupService(repo, &mockDumper{}, newMockObjectStore())
	svc.backupCipher = nil

	err := svc.RestoreBackup(context.Background(), "missing")
	require.ErrorIs(t, err, ErrBackupNotFound)
	_, err = svc.StartRestore(context.Background(), "missing")
	require.ErrorIs(t, err, ErrBackupNotFound)
}

func TestBackupService_RestoreLegacyBackupWithoutEncryptionKeyWhenEnabled(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "true")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	legacySQL := "SELECT 1;\n"
	legacy := seedLegacyBackupRecord(t, svc, store, "legacy-no-key", legacySQL)
	svc.backupCipher = nil

	require.NoError(t, svc.RestoreBackup(context.Background(), legacy.ID))
	require.Equal(t, legacySQL, string(dumper.restored))
}

func TestBackupService_RestoreLegacyBackupRequiresExplicitPolicy(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "false")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	legacy := seedLegacyBackupRecord(t, svc, store, "legacy-disabled", "SELECT 1;\n")

	err := svc.RestoreBackup(context.Background(), legacy.ID)
	require.ErrorIs(t, err, ErrLegacyBackupRestoreDisabled)
	require.Empty(t, dumper.restored)
}

func TestBackupService_RestoreLegacyBackupWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "true")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	legacySQL := "-- legacy\nSELECT 1;\n"
	legacy := seedLegacyBackupRecord(t, svc, store, "legacy-enabled", legacySQL)

	require.NoError(t, svc.RestoreBackup(context.Background(), legacy.ID))
	require.Equal(t, legacySQL, string(dumper.restored))
}

func TestBackupService_RestoreRejectsCorruptLegacyBeforeDumper(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "true")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	legacy := seedLegacyBackupRecord(t, svc, store, "legacy-corrupt", "SELECT 1;\n")
	store.mu.Lock()
	store.objects[legacy.S3Key][len(store.objects[legacy.S3Key])-1] ^= 0x80
	store.mu.Unlock()

	err := svc.RestoreBackup(context.Background(), legacy.ID)
	require.Error(t, err)
	require.Empty(t, dumper.restored)
}

func TestBackupService_RestoreRejectsUnknownFormatWithoutFallback(t *testing.T) {
	t.Setenv("SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE", "true")
	repo := newMockSettingRepo()
	seedS3Config(t, repo)
	dumper := &mockDumper{}
	store := newMockObjectStore()
	svc := newTestBackupService(repo, dumper, store)
	record := &BackupRecord{ID: "unknown", Status: "completed", Format: "future-v9", FileName: "future.sql.gz", S3Key: "backups/future.sql.gz"}
	require.NoError(t, svc.saveRecord(context.Background(), record))

	err := svc.RestoreBackup(context.Background(), record.ID)
	require.ErrorIs(t, err, ErrBackupFormatUnsupported)
	require.Empty(t, dumper.restored)
}

func seedLegacyBackupRecord(t *testing.T, svc *BackupService, store *mockObjectStore, id, sqlDump string) *BackupRecord {
	t.Helper()
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err := zw.Write([]byte(sqlDump))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	record := &BackupRecord{
		ID:         id,
		Status:     "completed",
		BackupType: "postgres",
		FileName:   id + ".sql.gz",
		S3Key:      "backups/" + id + ".sql.gz",
		SizeBytes:  int64(compressed.Len()),
		StartedAt:  time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	store.mu.Lock()
	store.objects[record.S3Key] = append([]byte(nil), compressed.Bytes()...)
	store.mu.Unlock()
	require.NoError(t, svc.saveRecord(context.Background(), record))
	return record
}
