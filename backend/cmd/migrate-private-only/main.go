package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/privatecutover"
	"github.com/Wei-Shaw/sub2api/internal/repository"
)

func main() {
	confirmation := flag.String("confirm", "", "required exact confirmation: DROP-SAAS-DATA-KEEP-USER-<lowest-active-admin-id>")
	localBackupDir := flag.String("local-backup-dir", os.Getenv("EXAPI_LOCAL_BACKUP_DIR"), "legacy local-backup directory whose pre-cutover files are purged after commit")
	noLocalBackups := flag.Bool("no-local-backups", false, "assert that the installation has no legacy local-backup directory")
	batchCleanupEvidencePath := flag.String("batch-cleanup-evidence-file", os.Getenv("EXAPI_BATCH_CLEANUP_EVIDENCE"), "required provider batch-cleanup attestation embedded in the signed report")
	reportPath := flag.String("report-file", os.Getenv("EXAPI_PRIVATE_MIGRATION_REPORT"), "required 0600 file for the durable signed migration report")
	flag.Parse()
	if strings.TrimSpace(*confirmation) == "" {
		log.Fatal("refusing to run without --confirm DROP-SAAS-DATA-KEEP-USER-<operator-id>")
	}
	if strings.TrimSpace(*reportPath) == "" {
		log.Fatal("refusing to run without --report-file (or EXAPI_PRIVATE_MIGRATION_REPORT)")
	}
	batchCleanupEvidence, err := privatecutover.ReadBatchCleanupEvidence(*batchCleanupEvidencePath)
	if err != nil {
		log.Fatal(err)
	}

	reportKey, err := privatecutover.ParseReportKey(os.Getenv("EXAPI_MIGRATION_REPORT_KEY"))
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
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close database client: %v", err)
		}
	}()

	if _, err := privatecutover.RunWithOptions(context.Background(), db, *confirmation, reportKey, privatecutover.CutoverOptions{
		LocalBackupDir:       *localBackupDir,
		AssertNoLocalBackups: *noLocalBackups,
		BatchCleanupEvidence: batchCleanupEvidence,
		ReportPath:           *reportPath,
	}, nil); err != nil {
		log.Fatalf("private-only cutover failed: %v", err)
	}
}
