package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
)

func main() {
	execute := flag.Bool("execute", false, "write encrypted bridge values; default verifies only")
	batchSize := flag.Int("batch-size", 250, "bounded migration batch size (1-5000)")
	cutover := flag.Bool("cutover", false, "clear compatible plaintext after verified migration")
	confirm := flag.String("confirm", "", "required value CIPHERTEXT_ONLY with --cutover")
	flag.Parse()
	if *cutover && (!*execute || *confirm != "CIPHERTEXT_ONLY") {
		log.Fatal("--cutover requires --execute --confirm CIPHERTEXT_ONLY and a verified snapshot/keyring backup")
	}

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	client, db, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer func() { _ = client.Close() }()

	stats, err := repository.MigrateSecretBridge(context.Background(), db, *batchSize, *execute)
	if err != nil {
		log.Fatalf("secret bridge migration failed: %v", err)
	}
	if *cutover {
		if err := repository.CutoverSecretBridge(context.Background(), db); err != nil {
			log.Fatalf("ciphertext-only cutover failed: %v", err)
		}
	}
	mode := "verify"
	if *execute {
		mode = "execute"
	}
	if *cutover {
		mode = "ciphertext-only-cutover"
	}
	fmt.Printf("mode=%s proxies_migrated=%d payment_configs_migrated=%d settings_migrated=%d admin_digests_migrated=%d legacy_proxies_remaining=%d legacy_payments_remaining=%d legacy_settings_remaining=%d\n",
		mode, stats.ProxiesMigrated, stats.PaymentConfigsMigrated, stats.SettingsMigrated,
		stats.AdminDigestsMigrated, stats.LegacyProxiesRemaining, stats.LegacyPaymentsRemaining,
		stats.LegacySettingsRemaining)
}
