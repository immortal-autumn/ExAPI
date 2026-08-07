package main

import (
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/server"
)

type Application struct {
	Servers     *server.HTTPServers
	PromptAudit *securityaudit.PromptService
	Cleanup     *shutdownCoordinator
}
