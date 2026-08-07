package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	settingKeyBackupS3Config = "backup_s3_config"
	settingKeyBackupSchedule = "backup_schedule"
	settingKeyBackupRecords  = "backup_records"
	backupFormatEncryptedV1  = "aes-256-gcm-stream-v1"
)

var (
	ErrBackupS3NotConfigured         = infraerrors.BadRequest("BACKUP_S3_NOT_CONFIGURED", "backup S3 storage is not configured")
	ErrBackupNotFound                = infraerrors.NotFound("BACKUP_NOT_FOUND", "backup record not found")
	ErrBackupInProgress              = infraerrors.Conflict("BACKUP_IN_PROGRESS", "a backup is already in progress")
	ErrRestoreInProgress             = infraerrors.Conflict("RESTORE_IN_PROGRESS", "a restore is already in progress")
	ErrBackupRecordsCorrupt          = infraerrors.InternalServer("BACKUP_RECORDS_CORRUPT", "backup records data is corrupted")
	ErrBackupS3ConfigCorrupt         = infraerrors.InternalServer("BACKUP_S3_CONFIG_CORRUPT", "backup S3 config data is corrupted")
	ErrBackupScheduleCorrupt         = infraerrors.InternalServer("BACKUP_SCHEDULE_CORRUPT", "backup schedule data is corrupted")
	ErrBackupEncryptionNotConfigured = infraerrors.ServiceUnavailable("BACKUP_ENCRYPTION_NOT_CONFIGURED", "backup encryption keyring is not configured")
	ErrLegacyBackupRestoreDisabled   = infraerrors.BadRequest("LEGACY_BACKUP_RESTORE_DISABLED", "legacy plaintext backup restore is disabled; set SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE=true for a controlled compatibility restore")
	ErrLiveRestoreDisabled           = infraerrors.Forbidden("LIVE_RESTORE_DISABLED", "live in-place restore is disabled in private production; restore into a disposable target")
	ErrBackupFormatUnsupported       = infraerrors.BadRequest("BACKUP_FORMAT_UNSUPPORTED", "backup format is unsupported")

	// ErrSecretEncryptionKeyNotConfigured is returned when an S3 SecretAccessKey
	// would be encrypted with an auto-generated (ephemeral) key. That key is
	// regenerated on every process start, so the persisted ciphertext becomes
	// undecryptable after a restart/upgrade ("cipher: message authentication
	// failed"), silently breaking S3 backup/image storage (#4524). Mirrors the
	// existing guards for payments (payment.ProvideEncryptionKey) and TOTP
	// enablement, which likewise refuse to depend on an auto-generated key.
	ErrSecretEncryptionKeyNotConfigured = infraerrors.BadRequest(
		"SECRET_ENCRYPTION_KEY_NOT_CONFIGURED",
		"cannot store the S3 secret access key: no fixed secret encryption key is configured, so the auto-generated key would change on every restart and make the stored secret undecryptable after a restart or upgrade. Set a fixed TOTP_ENCRYPTION_KEY (e.g. generate one with `openssl rand -hex 32`) and try again",
	)
)

// ─── 接口定义 ───

// DBDumper abstracts database dump/restore operations
type DBDumper interface {
	Dump(ctx context.Context) (io.ReadCloser, error)
	Restore(ctx context.Context, data io.Reader) error
}

// BackupObjectStore abstracts object storage for backup files
type BackupObjectStore interface {
	Upload(ctx context.Context, key string, body io.Reader, sizeBytes int64, contentType string) (uploadedBytes int64, err error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	HeadBucket(ctx context.Context) error
}

// BackupObjectStoreFactory creates an object store from S3 config
type BackupObjectStoreFactory func(ctx context.Context, cfg *BackupS3Config) (BackupObjectStore, error)

// ─── 数据模型 ───

// BackupS3Config S3 兼容存储配置（支持 Cloudflare R2）
type BackupS3Config struct {
	Endpoint        string `json:"endpoint"` // e.g. https://<account_id>.r2.cloudflarestorage.com
	Region          string `json:"region"`   // R2 用 "auto"
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key,omitempty"` //nolint:revive // field name follows AWS convention
	Prefix          string `json:"prefix"`                      // S3 key 前缀，如 "backups/"
	ForcePathStyle  bool   `json:"force_path_style"`
}

// IsConfigured 检查必要字段是否已配置
func (c *BackupS3Config) IsConfigured() bool {
	return c.Bucket != "" && c.AccessKeyID != "" && c.SecretAccessKey != ""
}

// BackupScheduleConfig 定时备份配置
type BackupScheduleConfig struct {
	Enabled     bool   `json:"enabled"`
	CronExpr    string `json:"cron_expr"`    // cron 表达式，如 "0 2 * * *" 每天凌晨2点
	RetainDays  int    `json:"retain_days"`  // 备份文件过期天数，默认14，0=不自动清理
	RetainCount int    `json:"retain_count"` // 最多保留份数，0=不限制
}

// BackupRecord 备份记录
type BackupRecord struct {
	ID            string `json:"id"`
	Format        string `json:"format,omitempty"` // empty only for legacy pre-encryption .sql.gz records
	Status        string `json:"status"`           // pending, running, completed, failed
	BackupType    string `json:"backup_type"`      // postgres
	FileName      string `json:"file_name"`
	S3Key         string `json:"s3_key"`
	SizeBytes     int64  `json:"size_bytes"`
	TriggeredBy   string `json:"triggered_by"` // manual, scheduled
	ErrorMsg      string `json:"error_message,omitempty"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`     // 过期时间
	Progress      string `json:"progress,omitempty"`       // "dumping", "uploading", ""
	RestoreStatus string `json:"restore_status,omitempty"` // "", "running", "completed", "failed"
	RestoreError  string `json:"restore_error,omitempty"`
	RestoredAt    string `json:"restored_at,omitempty"`
}

// BackupRecordForAPI returns the operational state without exposing internal
// subprocess, database, object-store, or credential diagnostics. Detailed
// errors remain server-side in persisted records and logs.
func BackupRecordForAPI(record BackupRecord) BackupRecord {
	record.ErrorMsg = ""
	record.RestoreError = ""
	record.FileName = ""
	record.S3Key = ""
	return record
}

func BackupRecordsForAPI(records []BackupRecord) []BackupRecord {
	out := make([]BackupRecord, len(records))
	for i := range records {
		out[i] = BackupRecordForAPI(records[i])
	}
	return out
}

// BackupService 数据库备份恢复服务
type BackupService struct {
	settingRepo SettingRepository
	dbCfg       *config.DatabaseConfig
	encryptor   SecretEncryptor
	// encryptionKeyConfigured mirrors cfg.Totp.EncryptionKeyConfigured: false
	// means the secret encryption key was auto-generated and does not survive a
	// restart. Durable-secret writers must refuse to persist new secrets in that
	// mode (#4524).
	encryptionKeyConfigured bool
	storeFactory            BackupObjectStoreFactory
	dumper                  DBDumper
	backupCipher            BackupStreamCipher
	liveRestoreDisabled     bool

	opMu      sync.Mutex // protects operation flags and admission versus shutdown
	backingUp bool
	restoring bool
	stopping  bool

	storeMu sync.Mutex // 保护 store/s3Cfg 缓存
	store   BackupObjectStore
	s3Cfg   *BackupS3Config

	recordsMu sync.Mutex // 保护 records 的 load/save 操作

	cronMu      sync.Mutex
	cronSched   *cron.Cron
	cronEntryID cron.EntryID
	started     bool

	wg           sync.WaitGroup     // 追踪活跃的备份/恢复 goroutine
	shuttingDown atomic.Bool        // 阻止新备份启动
	bgCtx        context.Context    // 所有后台操作的 parent context
	bgCancel     context.CancelFunc // 取消所有活跃后台操作
}

func NewBackupService(
	settingRepo SettingRepository,
	cfg *config.Config,
	encryptor SecretEncryptor,
	storeFactory BackupObjectStoreFactory,
	dumper DBDumper,
	backupCipher BackupStreamCipher,
) *BackupService {
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &BackupService{
		settingRepo:             settingRepo,
		dbCfg:                   &cfg.Database,
		encryptor:               encryptor,
		encryptionKeyConfigured: cfg.Totp.EncryptionKeyConfigured,
		storeFactory:            storeFactory,
		dumper:                  dumper,
		backupCipher:            backupCipher,
		liveRestoreDisabled:     config.SingleUserPrivateControlPlaneEnabled() && strings.EqualFold(strings.TrimSpace(cfg.Log.Environment), "production"),
		bgCtx:                   bgCtx,
		bgCancel:                bgCancel,
	}
}

// Start 启动定时备份调度器并清理孤立记录
func (s *BackupService) Start() {
	s.cronMu.Lock()
	if s.started || s.stopping || s.shuttingDown.Load() {
		s.cronMu.Unlock()
		return
	}
	s.started = true
	s.cronSched = cron.New()
	s.cronSched.Start()
	s.cronMu.Unlock()

	// Fail closed if stale operation state cannot be reconciled. Starting the
	// scheduler while records remain falsely running would hide interrupted work.
	if err := s.recoverStaleRecords(); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 恢复中断记录失败: %v", err)
		s.removeCronSchedule()
		return
	}

	// 加载已有的定时配置
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schedule, err := s.GetSchedule(ctx)
	if err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 加载定时备份配置失败: %v", err)
		return
	}
	if schedule.Enabled && schedule.CronExpr != "" {
		if err := s.applyCronSchedule(schedule); err != nil {
			logger.LegacyPrintf("service.backup", "[Backup] 应用定时备份配置失败: %v", err)
		}
	}
}

// recoverStaleRecords marks interrupted operations failed and reports any
// read/write failure so startup cannot pretend reconciliation succeeded.
func (s *BackupService) recoverStaleRecords() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	records, err := s.loadRecords(ctx)
	if err != nil {
		return err
	}
	for i := range records {
		if records[i].Status == "running" {
			// An uploading state means the external object may already exist. Preserve
			// retryable cleanup intent after a crash instead of creating an inert
			// failed record that retention ignores.
			if records[i].Progress == "uploading" && records[i].S3Key != "" {
				records[i].Status = "deleting"
				records[i].ErrorMsg = "interrupted during upload; object cleanup pending"
			} else {
				records[i].Status = "failed"
				records[i].ErrorMsg = "interrupted by server restart"
			}
			records[i].Progress = ""
			records[i].FinishedAt = time.Now().Format(time.RFC3339)
			if err := s.saveRecord(ctx, &records[i]); err != nil {
				return fmt.Errorf("persist stale backup recovery %s: %w", records[i].ID, err)
			}
			logger.LegacyPrintf("service.backup", "[Backup] recovered stale running record: %s", records[i].ID)
		}
		if records[i].RestoreStatus == "running" {
			records[i].RestoreStatus = "failed"
			records[i].RestoreError = "interrupted by server restart"
			if err := s.saveRecord(ctx, &records[i]); err != nil {
				return fmt.Errorf("persist stale restore recovery %s: %w", records[i].ID, err)
			}
			logger.LegacyPrintf("service.backup", "[Backup] recovered stale restoring record: %s", records[i].ID)
		}
	}
	return nil
}

// Stop stops scheduling and waits for active backup/restore operations until
// the supplied context expires. Cancellation is propagated immediately.
func (s *BackupService) StopContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	s.stopping = true
	s.shuttingDown.Store(true)
	s.opMu.Unlock()

	s.cronMu.Lock()
	if s.cronSched != nil {
		s.cronSched.Stop()
	}
	s.cronMu.Unlock()

	if s.bgCancel != nil {
		s.bgCancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.LegacyPrintf("service.backup", "[Backup] all active operations finished")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("backup shutdown deadline: %w", ctx.Err())
	}
}

// Stop preserves the standalone service API while application cleanup uses
// StopContext with its explicit global deadline.
func (s *BackupService) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := s.StopContext(ctx); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] shutdown incomplete: %v", err)
	}
}

// ─── S3 配置管理 ───

// EncryptionKeyConfigured reports whether a fixed (explicitly configured) secret
// encryption key is in use. When false the key is auto-generated on every start
// and secrets encrypted with it cannot be recovered after a restart, so callers
// that persist durable secrets must refuse to do so (#4524).
func (s *BackupService) EncryptionKeyConfigured() bool {
	return s != nil && s.encryptionKeyConfigured
}

func (s *BackupService) GetS3Config(ctx context.Context) (*BackupS3Config, error) {
	cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return &BackupS3Config{}, nil
	}
	// 脱敏返回
	cfg.SecretAccessKey = ""
	return cfg, nil
}

func (s *BackupService) UpdateS3Config(ctx context.Context, cfg BackupS3Config) (*BackupS3Config, error) {
	hasReplacementSecret := cfg.SecretAccessKey != ""
	// If no replacement is supplied, decrypt the existing value and immediately
	// re-encrypt it before persistence. Persisting the decrypted value would turn
	// an ordinary metadata-only update into a plaintext credential write.
	if cfg.SecretAccessKey == "" {
		old, err := s.loadS3Config(ctx)
		if err != nil {
			return nil, fmt.Errorf("load existing S3 config: %w", err)
		}
		if old != nil {
			cfg.SecretAccessKey = old.SecretAccessKey
		}
	}
	if hasReplacementSecret && !s.encryptionKeyConfigured {
		return nil, ErrSecretEncryptionKeyNotConfigured
	}
	if cfg.SecretAccessKey != "" {
		encrypted, err := s.encryptor.Encrypt(cfg.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret: %w", err)
		}
		cfg.SecretAccessKey = encrypted
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal s3 config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyBackupS3Config, string(data)); err != nil {
		return nil, fmt.Errorf("save s3 config: %w", err)
	}

	// 清除缓存的 S3 客户端
	s.storeMu.Lock()
	s.store = nil
	s.s3Cfg = nil
	s.storeMu.Unlock()

	cfg.SecretAccessKey = ""
	return &cfg, nil
}

func (s *BackupService) TestS3Connection(ctx context.Context, cfg BackupS3Config) error {
	// 如果没提供 secret，用已保存的
	if cfg.SecretAccessKey == "" {
		old, err := s.loadS3Config(ctx)
		if err != nil {
			return fmt.Errorf("load existing S3 config: %w", err)
		}
		if old != nil {
			cfg.SecretAccessKey = old.SecretAccessKey
		}
	}

	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return fmt.Errorf("S3 配置不完整：必须填写存储桶、Access Key ID 和 Secret Access Key")
	}

	store, err := s.storeFactory(ctx, &cfg)
	if err != nil {
		return err
	}
	return store.HeadBucket(ctx)
}

// ─── 定时备份管理 ───

func (s *BackupService) GetSchedule(ctx context.Context) (*BackupScheduleConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, settingKeyBackupSchedule)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &BackupScheduleConfig{}, nil
		}
		return nil, err
	}
	if raw == "" {
		return &BackupScheduleConfig{}, nil
	}
	var cfg BackupScheduleConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, ErrBackupScheduleCorrupt
	}
	return &cfg, nil
}

func (s *BackupService) UpdateSchedule(ctx context.Context, cfg BackupScheduleConfig) (*BackupScheduleConfig, error) {
	if cfg.RetainDays < 0 || cfg.RetainCount < 0 {
		return nil, infraerrors.BadRequest("INVALID_RETENTION", "retention days and count must be non-negative")
	}
	if cfg.Enabled && cfg.CronExpr == "" {
		return nil, infraerrors.BadRequest("INVALID_CRON", "cron expression is required when schedule is enabled")
	}
	// 验证 cron 表达式
	if cfg.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(cfg.CronExpr); err != nil {
			return nil, infraerrors.BadRequest("INVALID_CRON", fmt.Sprintf("invalid cron expression: %v", err))
		}
	}

	// Validate runtime applicability before persisting an enabled schedule. This
	// avoids durable config claiming the scheduler is active when installation
	// cannot succeed.
	if cfg.Enabled {
		s.cronMu.Lock()
		schedulerPresent := s.cronSched != nil
		s.cronMu.Unlock()
		s.opMu.Lock()
		stopping := s.stopping || s.shuttingDown.Load()
		s.opMu.Unlock()
		if !schedulerPresent || stopping {
			return nil, fmt.Errorf("cron scheduler is not available")
		}
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal schedule config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyBackupSchedule, string(data)); err != nil {
		return nil, fmt.Errorf("save schedule config: %w", err)
	}

	// 应用或停止定时任务
	if cfg.Enabled {
		if err := s.applyCronSchedule(&cfg); err != nil {
			return nil, err
		}
	} else {
		s.removeCronSchedule()
	}

	return &cfg, nil
}

func (s *BackupService) applyCronSchedule(cfg *BackupScheduleConfig) error {
	s.cronMu.Lock()
	defer s.cronMu.Unlock()

	if s.cronSched == nil {
		return fmt.Errorf("cron scheduler not initialized")
	}

	// 移除旧任务
	if s.cronEntryID != 0 {
		s.cronSched.Remove(s.cronEntryID)
		s.cronEntryID = 0
	}

	entryID, err := s.cronSched.AddFunc(cfg.CronExpr, func() {
		s.runScheduledBackup()
	})
	if err != nil {
		return infraerrors.BadRequest("INVALID_CRON", fmt.Sprintf("failed to schedule: %v", err))
	}
	s.cronEntryID = entryID
	logger.LegacyPrintf("service.backup", "[Backup] 定时备份已启用: %s", cfg.CronExpr)
	return nil
}

func (s *BackupService) removeCronSchedule() {
	s.cronMu.Lock()
	defer s.cronMu.Unlock()
	if s.cronSched != nil && s.cronEntryID != 0 {
		s.cronSched.Remove(s.cronEntryID)
		s.cronEntryID = 0
		logger.LegacyPrintf("service.backup", "[Backup] 定时备份已停用")
	}
}

func (s *BackupService) runScheduledBackup() {
	if !s.admitBackgroundOperation() {
		return
	}
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(s.bgCtx, 30*time.Minute)
	defer cancel()

	// Read the retention policy fail-closed. A repository/corruption error must
	// not silently substitute defaults and create backups outside operator policy.
	schedule, err := s.GetSchedule(ctx)
	if err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 读取备份保留策略失败: %v", err)
		return
	}
	expireDays := 14 // 默认14天过期
	if schedule != nil && schedule.RetainDays > 0 {
		expireDays = schedule.RetainDays
	}

	logger.LegacyPrintf("service.backup", "[Backup] 开始执行定时备份, 过期天数: %d", expireDays)
	record, err := s.createBackup(ctx, "scheduled", expireDays)
	if err != nil {
		if errors.Is(err, ErrBackupInProgress) {
			logger.LegacyPrintf("service.backup", "[Backup] 定时备份跳过: 已有备份正在进行中")
		} else {
			logger.LegacyPrintf("service.backup", "[Backup] 定时备份失败: %v", err)
		}
		return
	}
	logger.LegacyPrintf("service.backup", "[Backup] 定时备份完成: id=%s size=%d", record.ID, record.SizeBytes)

	// 清理过期备份（复用已加载的 schedule）
	if schedule == nil {
		return
	}
	if err := s.cleanupOldBackups(ctx, schedule); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 清理过期备份失败: %v", err)
	}
}

// ─── 备份/恢复核心 ───

func (s *BackupService) requireBackupEncryption() error {
	if s.backupCipher == nil {
		return ErrBackupEncryptionNotConfigured
	}
	return nil
}

func (s *BackupService) admitBackgroundOperation() bool {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.stopping || s.shuttingDown.Load() {
		return false
	}
	s.wg.Add(1)
	return true
}

func (s *BackupService) serverShuttingDownError() error {
	return infraerrors.ServiceUnavailable("SERVER_SHUTTING_DOWN", "server is shutting down")
}

// CreateBackup 创建全量数据库备份并上传到 S3（流式处理）
// expireDays: 备份过期天数，0=永不过期，默认14天
func (s *BackupService) CreateBackup(ctx context.Context, triggeredBy string, expireDays int) (*BackupRecord, error) {
	if expireDays < 0 {
		return nil, infraerrors.BadRequest("INVALID_RETENTION", "backup expiry days must be non-negative")
	}
	if !s.admitBackgroundOperation() {
		return nil, s.serverShuttingDownError()
	}
	defer s.wg.Done()
	return s.createBackup(ctx, triggeredBy, expireDays)
}

func (s *BackupService) createBackup(ctx context.Context, triggeredBy string, expireDays int) (*BackupRecord, error) {
	if err := s.requireBackupEncryption(); err != nil {
		return nil, err
	}

	s.opMu.Lock()
	if s.backingUp {
		s.opMu.Unlock()
		return nil, ErrBackupInProgress
	}
	if s.restoring {
		s.opMu.Unlock()
		return nil, ErrRestoreInProgress
	}
	s.backingUp = true
	s.opMu.Unlock()
	defer func() {
		s.opMu.Lock()
		s.backingUp = false
		s.opMu.Unlock()
	}()

	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return nil, err
	}
	if s3Cfg == nil || !s3Cfg.IsConfigured() {
		return nil, ErrBackupS3NotConfigured
	}

	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return nil, fmt.Errorf("init object store: %w", err)
	}

	now := time.Now()
	backupID := uuid.New().String()[:8]
	fileName := fmt.Sprintf("%s_%s.sql.gz.enc", s.dbCfg.DBName, now.Format("20060102_150405"))
	s3Key := s.buildS3Key(s3Cfg, fileName, backupID)

	var expiresAt string
	if expireDays > 0 {
		expiresAt = now.AddDate(0, 0, expireDays).Format(time.RFC3339)
	}

	record := &BackupRecord{
		ID:          backupID,
		Format:      backupFormatEncryptedV1,
		Status:      "running",
		BackupType:  "postgres",
		FileName:    fileName,
		S3Key:       s3Key,
		TriggeredBy: triggeredBy,
		StartedAt:   now.Format(time.RFC3339),
		ExpiresAt:   expiresAt,
		Progress:    "pending",
	}

	// Make the object discoverable before any irreversible upload. If metadata
	// cannot be persisted, fail before producing an orphaned encrypted object.
	if err := s.saveRecord(ctx, record); err != nil {
		return record, fmt.Errorf("save initial backup record: %w", err)
	}

	// Mark possible external-object creation durably before dump/encryption/upload.
	// Startup reconciliation treats this phase as cleanup-pending after a crash.
	record.Progress = "uploading"
	if err := s.saveRecord(ctx, record); err != nil {
		return record, fmt.Errorf("persist uploading backup state: %w", err)
	}

	// pg_dump -> gzip -> authenticated encrypted staging file -> object storage.
	sizeBytes, err := s.uploadEncryptedDump(ctx, objectStore, s3Key)
	if err != nil {
		record.Status = "failed"
		record.Progress = ""
		record.ErrorMsg = fmt.Sprintf("encrypted backup failed: %v", err)
		record.FinishedAt = time.Now().Format(time.RFC3339)
		if persistErr := s.saveRecord(ctx, record); persistErr != nil {
			return record, errors.Join(err, fmt.Errorf("persist failed backup metadata: %w", persistErr))
		}
		return record, err
	}

	record.SizeBytes = sizeBytes
	record.Status = "completed"
	record.Progress = ""
	record.FinishedAt = time.Now().Format(time.RFC3339)
	if err := s.saveRecord(ctx, record); err != nil {
		// Completion compensation must not reuse an expired request/operation
		// context. Give object cleanup and tombstone persistence a small bounded
		// detached window.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		persistErr := err
		if deleteErr := objectStore.Delete(cleanupCtx, s3Key); deleteErr != nil {
			record.Status = "deleting"
			record.Progress = ""
			record.ErrorMsg = "backup completion metadata failed and object cleanup is pending"
			record.FinishedAt = time.Now().Format(time.RFC3339)
			if tombstoneErr := s.saveRecord(cleanupCtx, record); tombstoneErr != nil {
				return record, errors.Join(
					fmt.Errorf("save completed backup record: %w", persistErr),
					fmt.Errorf("compensate uploaded backup object: %w", deleteErr),
					fmt.Errorf("persist cleanup tombstone: %w", tombstoneErr),
				)
			}
			return record, errors.Join(
				fmt.Errorf("save completed backup record: %w", persistErr),
				fmt.Errorf("compensate uploaded backup object: %w", deleteErr),
			)
		}
		record.Status = "failed"
		record.Progress = ""
		record.ErrorMsg = "backup upload was rolled back because completion metadata could not be persisted"
		record.FinishedAt = time.Now().Format(time.RFC3339)
		if failureErr := s.saveRecord(cleanupCtx, record); failureErr != nil {
			return record, errors.Join(
				fmt.Errorf("save completed backup record: %w", persistErr),
				fmt.Errorf("persist compensated backup state: %w", failureErr),
			)
		}
		return record, fmt.Errorf("save completed backup record: %w", persistErr)
	}

	return record, nil
}

// StartBackup 异步创建备份，立即返回 running 状态的记录
func (s *BackupService) StartBackup(ctx context.Context, triggeredBy string, expireDays int) (*BackupRecord, error) {
	if expireDays < 0 {
		return nil, infraerrors.BadRequest("INVALID_RETENTION", "backup expiry days must be non-negative")
	}
	if s.shuttingDown.Load() {
		return nil, infraerrors.ServiceUnavailable("SERVER_SHUTTING_DOWN", "server is shutting down")
	}
	if err := s.requireBackupEncryption(); err != nil {
		return nil, err
	}

	s.opMu.Lock()
	if s.stopping || s.shuttingDown.Load() {
		s.opMu.Unlock()
		return nil, s.serverShuttingDownError()
	}
	if s.backingUp {
		s.opMu.Unlock()
		return nil, ErrBackupInProgress
	}
	if s.restoring {
		s.opMu.Unlock()
		return nil, ErrRestoreInProgress
	}
	s.backingUp = true
	s.wg.Add(1)
	s.opMu.Unlock()

	// Initialization failure releases both the operation flag and admission.
	launched := false
	defer func() {
		if !launched {
			s.opMu.Lock()
			s.backingUp = false
			s.opMu.Unlock()
			s.wg.Done()
		}
	}()

	// 在返回前加载 S3 配置和创建 store，避免 goroutine 中配置被修改
	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return nil, err
	}
	if s3Cfg == nil || !s3Cfg.IsConfigured() {
		return nil, ErrBackupS3NotConfigured
	}

	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return nil, fmt.Errorf("init object store: %w", err)
	}

	now := time.Now()
	backupID := uuid.New().String()[:8]
	fileName := fmt.Sprintf("%s_%s.sql.gz.enc", s.dbCfg.DBName, now.Format("20060102_150405"))
	s3Key := s.buildS3Key(s3Cfg, fileName, backupID)

	var expiresAt string
	if expireDays > 0 {
		expiresAt = now.AddDate(0, 0, expireDays).Format(time.RFC3339)
	}

	record := &BackupRecord{
		ID:          backupID,
		Format:      backupFormatEncryptedV1,
		Status:      "running",
		BackupType:  "postgres",
		FileName:    fileName,
		S3Key:       s3Key,
		TriggeredBy: triggeredBy,
		StartedAt:   now.Format(time.RFC3339),
		ExpiresAt:   expiresAt,
		Progress:    "pending",
	}

	if err := s.saveRecord(ctx, record); err != nil {
		return nil, fmt.Errorf("save initial record: %w", err)
	}

	launched = true
	// 在启动 goroutine 前完成拷贝，避免数据竞争
	result := *record

	go func() {
		defer s.wg.Done()
		defer func() {
			s.opMu.Lock()
			s.backingUp = false
			s.opMu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				logger.LegacyPrintf("service.backup", "[Backup] panic recovered: %v", r)
				record.Status = "failed"
				record.ErrorMsg = fmt.Sprintf("internal panic: %v", r)
				record.Progress = ""
				record.FinishedAt = time.Now().Format(time.RFC3339)
				_ = s.saveRecord(context.Background(), record)
			}
		}()
		s.executeBackup(record, objectStore)
	}()

	return &result, nil
}

func (s *BackupService) persistAsyncBackupFailure(record *BackupRecord, message string) {
	record.Status = "failed"
	record.ErrorMsg = message
	record.Progress = ""
	record.FinishedAt = time.Now().Format(time.RFC3339)
	if err := s.saveRecord(context.Background(), record); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 保存异步失败状态失败: %v", err)
	}
}

// executeBackup 后台执行备份（独立于 HTTP context）
func (s *BackupService) executeBackup(record *BackupRecord, objectStore BackupObjectStore) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 30*time.Minute)
	defer cancel()

	record.Progress = "dumping"
	if err := s.saveRecord(ctx, record); err != nil {
		s.persistAsyncBackupFailure(record, fmt.Sprintf("persist dumping progress: %v", err))
		return
	}

	// Shared authenticated staging path: pg_dump -> gzip -> encrypted file -> upload.
	record.Progress = "uploading"
	if err := s.saveRecord(ctx, record); err != nil {
		s.persistAsyncBackupFailure(record, fmt.Sprintf("persist uploading progress: %v", err))
		return
	}
	sizeBytes, err := s.uploadEncryptedDump(ctx, objectStore, record.S3Key)
	if err != nil {
		record.Status = "failed"
		record.ErrorMsg = fmt.Sprintf("encrypted backup failed: %v", err)
		record.Progress = ""
		record.FinishedAt = time.Now().Format(time.RFC3339)
		_ = s.saveRecord(context.Background(), record)
		return
	}

	record.SizeBytes = sizeBytes
	record.Status = "completed"
	record.Progress = ""
	record.FinishedAt = time.Now().Format(time.RFC3339)
	completionCtx, completionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer completionCancel()
	if err := s.saveRecord(completionCtx, record); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 保存完成状态失败: %v", err)
		if deleteErr := objectStore.Delete(completionCtx, record.S3Key); deleteErr != nil {
			logger.LegacyPrintf("service.backup", "[Backup] 回滚已上传对象失败: %v", deleteErr)
			record.Status = "deleting"
			record.ErrorMsg = "backup completion metadata failed and object cleanup is pending"
		} else {
			record.Status = "failed"
			record.ErrorMsg = "backup upload was rolled back because completion metadata could not be persisted"
		}
		record.Progress = ""
		if retryErr := s.saveRecord(completionCtx, record); retryErr != nil {
			logger.LegacyPrintf("service.backup", "[Backup] 保存补偿后元数据失败状态失败: %v", retryErr)
		}
	}
}

// RestoreBackup 从 S3 下载备份并流式恢复到数据库
func (s *BackupService) RestoreBackup(ctx context.Context, backupID string) error {
	if s.liveRestoreDisabled {
		return ErrLiveRestoreDisabled
	}
	if !s.admitBackgroundOperation() {
		return s.serverShuttingDownError()
	}
	defer s.wg.Done()
	s.opMu.Lock()
	if s.restoring {
		s.opMu.Unlock()
		return ErrRestoreInProgress
	}
	if s.backingUp {
		s.opMu.Unlock()
		return ErrBackupInProgress
	}
	s.restoring = true
	s.opMu.Unlock()
	defer func() {
		s.opMu.Lock()
		s.restoring = false
		s.opMu.Unlock()
	}()

	record, err := s.GetBackupRecord(ctx, backupID)
	if err != nil {
		return err
	}
	if record.Status != "completed" {
		return infraerrors.BadRequest("BACKUP_NOT_COMPLETED", "can only restore from a completed backup")
	}

	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return err
	}
	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return fmt.Errorf("init object store: %w", err)
	}

	return s.restoreBackupRecord(ctx, objectStore, record)
}

// StartRestore 异步恢复备份，立即返回
func (s *BackupService) StartRestore(ctx context.Context, backupID string) (*BackupRecord, error) {
	if s.liveRestoreDisabled {
		return nil, ErrLiveRestoreDisabled
	}
	if s.shuttingDown.Load() {
		return nil, infraerrors.ServiceUnavailable("SERVER_SHUTTING_DOWN", "server is shutting down")
	}

	s.opMu.Lock()
	if s.stopping || s.shuttingDown.Load() {
		s.opMu.Unlock()
		return nil, s.serverShuttingDownError()
	}
	if s.restoring {
		s.opMu.Unlock()
		return nil, ErrRestoreInProgress
	}
	if s.backingUp {
		s.opMu.Unlock()
		return nil, ErrBackupInProgress
	}
	s.restoring = true
	s.wg.Add(1)
	s.opMu.Unlock()

	// Initialization failure releases both the operation flag and admission.
	launched := false
	defer func() {
		if !launched {
			s.opMu.Lock()
			s.restoring = false
			s.opMu.Unlock()
			s.wg.Done()
		}
	}()

	record, err := s.GetBackupRecord(ctx, backupID)
	if err != nil {
		return nil, err
	}
	if record.Status != "completed" {
		return nil, infraerrors.BadRequest("BACKUP_NOT_COMPLETED", "can only restore from a completed backup")
	}

	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return nil, err
	}
	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return nil, fmt.Errorf("init object store: %w", err)
	}

	record.RestoreStatus = "running"
	if err := s.saveRecord(ctx, record); err != nil {
		return nil, fmt.Errorf("save initial restore status: %w", err)
	}

	launched = true
	result := *record

	go func() {
		defer s.wg.Done()
		defer func() {
			s.opMu.Lock()
			s.restoring = false
			s.opMu.Unlock()
		}()
		defer func() {
			if r := recover(); r != nil {
				logger.LegacyPrintf("service.backup", "[Backup] restore panic recovered: %v", r)
				record.RestoreStatus = "failed"
				record.RestoreError = fmt.Sprintf("internal panic: %v", r)
				_ = s.saveRecord(context.Background(), record)
			}
		}()
		s.executeRestore(record, objectStore)
	}()

	return &result, nil
}

// executeRestore 后台执行恢复
func (s *BackupService) executeRestore(record *BackupRecord, objectStore BackupObjectStore) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 30*time.Minute)
	defer cancel()

	if err := s.restoreBackupRecord(ctx, objectStore, record); err != nil {
		record.RestoreStatus = "failed"
		record.RestoreError = fmt.Sprintf("authenticated restore failed: %v", err)
		_ = s.saveRecord(context.Background(), record)
		return
	}

	record.RestoreStatus = "completed"
	record.RestoredAt = time.Now().Format(time.RFC3339)
	if err := s.saveRecord(context.Background(), record); err != nil {
		logger.LegacyPrintf("service.backup", "[Backup] 保存恢复完成状态失败: %v", err)
		record.RestoreStatus = "failed"
		record.RestoreError = fmt.Sprintf("persist completed restore metadata: %v", err)
		if retryErr := s.saveRecord(context.Background(), record); retryErr != nil {
			logger.LegacyPrintf("service.backup", "[Backup] 保存恢复元数据失败状态失败: %v", retryErr)
		}
	}
}

// ─── 备份记录管理 ───

func (s *BackupService) ListBackups(ctx context.Context) ([]BackupRecord, error) {
	records, err := s.loadRecords(ctx)
	if err != nil {
		return nil, err
	}
	// 倒序返回（最新在前）
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt > records[j].StartedAt
	})
	return records, nil
}

func (s *BackupService) GetBackupRecord(ctx context.Context, backupID string) (*BackupRecord, error) {
	records, err := s.loadRecords(ctx)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].ID == backupID {
			return &records[i], nil
		}
	}
	return nil, ErrBackupNotFound
}

func (s *BackupService) DeleteBackup(ctx context.Context, backupID string) error {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()

	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return err
	}

	foundIndex := -1
	for i := range records {
		if records[i].ID == backupID {
			foundIndex = i
			break
		}
	}
	if foundIndex < 0 {
		return ErrBackupNotFound
	}

	// Persist deletion intent before touching object storage. A surviving
	// tombstone makes retries safe when object deletion succeeds but metadata
	// removal fails.
	if records[foundIndex].Status != "deleting" {
		records[foundIndex].Status = "deleting"
		if err := s.saveRecordsLocked(ctx, records); err != nil {
			return fmt.Errorf("persist backup deletion intent: %w", err)
		}
	}
	found := records[foundIndex]
	if found.S3Key != "" {
		s3Cfg, err := s.loadS3Config(ctx)
		if err != nil {
			return fmt.Errorf("load S3 config for delete: %w", err)
		}
		if s3Cfg == nil || !s3Cfg.IsConfigured() {
			return ErrBackupS3NotConfigured
		}
		objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
		if err != nil {
			return fmt.Errorf("init object store for delete: %w", err)
		}
		if err := objectStore.Delete(ctx, found.S3Key); err != nil {
			return fmt.Errorf("delete backup object: %w", err)
		}
	}

	remaining := append(records[:foundIndex], records[foundIndex+1:]...)
	if err := s.saveRecordsLocked(ctx, remaining); err != nil {
		return fmt.Errorf("persist backup metadata removal: %w", err)
	}
	return nil
}

// GetBackupDownloadURL 获取备份文件预签名下载 URL
func (s *BackupService) GetBackupDownloadURL(ctx context.Context, backupID string) (string, error) {
	record, err := s.GetBackupRecord(ctx, backupID)
	if err != nil {
		return "", err
	}
	if record.Status != "completed" {
		return "", infraerrors.BadRequest("BACKUP_NOT_COMPLETED", "backup is not completed")
	}

	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return "", err
	}
	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return "", err
	}

	url, err := objectStore.PresignURL(ctx, record.S3Key, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return url, nil
}

// ─── 内部方法 ───

func (s *BackupService) loadS3Config(ctx context.Context) (*BackupS3Config, error) {
	raw, err := s.settingRepo.GetValue(ctx, settingKeyBackupS3Config)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil //nolint:nilnil // an absent setting means S3 is not configured yet
		}
		return nil, err
	}
	if raw == "" {
		return nil, nil //nolint:nilnil // no config is a valid state
	}
	var cfg BackupS3Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, ErrBackupS3ConfigCorrupt
	}
	// Stored credentials must be encrypted. Treat decryption failures as corrupt
	// secret-bearing configuration rather than silently using plaintext.
	if cfg.SecretAccessKey != "" {
		decrypted, err := s.encryptor.Decrypt(cfg.SecretAccessKey)
		if err != nil {
			return nil, fmt.Errorf("%w: decrypt stored S3 credential", ErrBackupS3ConfigCorrupt)
		}
		cfg.SecretAccessKey = decrypted
	}
	return &cfg, nil
}

func (s *BackupService) getOrCreateStore(ctx context.Context, cfg *BackupS3Config) (BackupObjectStore, error) {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()

	if s.store != nil && s.s3Cfg != nil && backupS3ConfigEqual(s.s3Cfg, cfg) {
		return s.store, nil
	}

	if cfg == nil {
		return nil, ErrBackupS3NotConfigured
	}

	store, err := s.storeFactory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	cfgCopy := *cfg
	s.store = store
	s.s3Cfg = &cfgCopy
	return store, nil
}

func backupS3ConfigEqual(a, b *BackupS3Config) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Endpoint == b.Endpoint &&
		a.Region == b.Region &&
		a.Bucket == b.Bucket &&
		a.AccessKeyID == b.AccessKeyID &&
		a.SecretAccessKey == b.SecretAccessKey &&
		a.Prefix == b.Prefix &&
		a.ForcePathStyle == b.ForcePathStyle
}

func (s *BackupService) buildS3Key(cfg *BackupS3Config, fileName, backupID string) string {
	prefix := strings.TrimRight(cfg.Prefix, "/")
	if prefix == "" {
		prefix = "backups"
	}
	return fmt.Sprintf("%s/%s/%s-%s", prefix, time.Now().Format("2006/01/02"), backupID, fileName)
}

// loadRecords 加载备份记录，区分"无数据"和"数据损坏"
func (s *BackupService) loadRecords(ctx context.Context) ([]BackupRecord, error) {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	return s.loadRecordsLocked(ctx)
}

// loadRecordsLocked 在已持有 recordsMu 锁的情况下加载记录
func (s *BackupService) loadRecordsLocked(ctx context.Context) ([]BackupRecord, error) {
	raw, err := s.settingRepo.GetValue(ctx, settingKeyBackupRecords)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil //nolint:nilnil // an absent setting means no backup history yet
		}
		return nil, err
	}
	if raw == "" {
		return nil, nil //nolint:nilnil // no records is a valid state
	}
	var records []BackupRecord
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		return nil, ErrBackupRecordsCorrupt
	}
	return records, nil
}

// saveRecordsLocked 在已持有 recordsMu 锁的情况下保存记录
func (s *BackupService) saveRecordsLocked(ctx context.Context, records []BackupRecord) error {
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, settingKeyBackupRecords, string(data))
}

// saveRecord 保存单条记录（带互斥锁保护）
func (s *BackupService) saveRecord(ctx context.Context, record *BackupRecord) error {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()

	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return err
	}

	// 更新已有记录或追加
	found := false
	for i := range records {
		if records[i].ID == record.ID {
			records[i] = *record
			found = true
			break
		}
	}
	if !found {
		records = append(records, *record)
	}

	return s.saveRecordsLocked(ctx, records)
}

func (s *BackupService) cleanupOldBackups(ctx context.Context, schedule *BackupScheduleConfig) error {
	if schedule == nil {
		return nil
	}

	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()

	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return err
	}

	// Sort newest first for count-based retention. A durable "deleting"
	// tombstone bridges object deletion and metadata removal: if the latter fails,
	// the next cleanup safely retries the idempotent object delete and finishes
	// removing the record instead of leaving a false restorable entry.
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt > records[j].StartedAt
	})

	candidateIDs := make([]string, 0)
	completedSeen := 0
	for _, r := range records {
		shouldDelete := r.Status == "deleting"
		if r.Status == "completed" {
			completedSeen++
			if schedule.RetainCount > 0 && completedSeen > schedule.RetainCount {
				shouldDelete = true
			}
			if schedule.RetainDays > 0 && r.StartedAt != "" {
				startedAt, parseErr := time.Parse(time.RFC3339, r.StartedAt)
				if parseErr != nil {
					return fmt.Errorf("invalid backup started_at for %s: %w", r.ID, parseErr)
				}
				if time.Since(startedAt) > time.Duration(schedule.RetainDays)*24*time.Hour {
					shouldDelete = true
				}
			}
		}
		if shouldDelete {
			candidateIDs = append(candidateIDs, r.ID)
		}
	}

	deletedCount := 0
	for _, id := range candidateIDs {
		idx := -1
		for i := range records {
			if records[i].ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		record := records[idx]
		if record.Status != "deleting" {
			records[idx].Status = "deleting"
			if err := s.saveRecordsLocked(ctx, records); err != nil {
				return fmt.Errorf("persist backup deletion intent %s: %w", id, err)
			}
		}
		if record.S3Key != "" {
			if err := s.deleteS3Object(ctx, record.S3Key); err != nil {
				// Keep the durable tombstone so a later run retries safely.
				return fmt.Errorf("delete expired backup %s: %w", id, err)
			}
		}
		records = append(records[:idx], records[idx+1:]...)
		if err := s.saveRecordsLocked(ctx, records); err != nil {
			return fmt.Errorf("persist backup retention progress: %w", err)
		}
		deletedCount++
	}
	if deletedCount > 0 {
		logger.LegacyPrintf("service.backup", "[Backup] 自动清理了 %d 个过期备份", deletedCount)
	}
	return nil
}

func (s *BackupService) deleteS3Object(ctx context.Context, key string) error {
	s3Cfg, err := s.loadS3Config(ctx)
	if err != nil {
		return err
	}
	if s3Cfg == nil {
		return ErrBackupS3NotConfigured
	}
	objectStore, err := s.getOrCreateStore(ctx, s3Cfg)
	if err != nil {
		return err
	}
	return objectStore.Delete(ctx, key)
}
