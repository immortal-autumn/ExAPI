package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/privatecutover"
	"github.com/Wei-Shaw/sub2api/internal/repository"

	_ "github.com/lib/pq"
)

func main() {
	confirmation := flag.String("confirm", "", "required exact confirmation: DROP-SAAS-DATA-KEEP-USER-<lowest-active-admin-id>")
	backupDir := flag.String("backup-dir", os.Getenv("EXAPI_BACKUP_DIR"), "directory whose pre-cutover backups are purged after commit")
	reportPath := flag.String("report-file", os.Getenv("EXAPI_PRIVATE_MIGRATION_REPORT"), "write the signed migration report to this 0600 file")
	flag.Parse()
	if strings.TrimSpace(*confirmation) == "" {
		log.Fatal("refusing to run without --confirm DROP-SAAS-DATA-KEEP-USER-<operator-id>")
	}

	reportKey, err := migrationReportKey()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	client, db, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer client.Close()

	var report io.Writer = os.Stdout
	if strings.TrimSpace(*reportPath) != "" {
		file, err := os.OpenFile(*reportPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			log.Fatalf("open report file: %v", err)
		}
		defer file.Close()
		report = file
	}
	if _, err := privatecutover.Run(context.Background(), db, *confirmation, reportKey, *backupDir, nil, report); err != nil {
		log.Fatalf("private-only cutover failed: %v", err)
	}
}

func migrationReportKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("EXAPI_MIGRATION_REPORT_KEY"))
	if raw == "" {
		return nil, fmt.Errorf("EXAPI_MIGRATION_REPORT_KEY is required and must be a 32-byte key or hex encoding")
	}
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) >= 32 {
		return decoded, nil
	}
	if len([]byte(raw)) >= 32 {
		return []byte(raw), nil
	}
	return nil, fmt.Errorf("EXAPI_MIGRATION_REPORT_KEY must be at least 32 bytes (sha256 fingerprint=%s)", keyFingerprint([]byte(raw)))
}

func keyFingerprint(key []byte) string {
	digest := sha256.Sum256(key)
	return hex.EncodeToString(digest[:])[:16]
}
