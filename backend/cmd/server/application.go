package main

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
)

type Application struct {
	Server      *http.Server
	PromptAudit *securityaudit.PromptService
	Cleanup     *shutdownCoordinator
}
