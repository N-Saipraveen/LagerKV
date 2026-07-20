package main

import (
	"github.com/N-Saipraveen/LagerKV/pkg/config"
	"github.com/N-Saipraveen/LagerKV/pkg/logger"
	"github.com/N-Saipraveen/LagerKV/server"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		logger.Module("lagerkv-server"),
		config.Module(),
		server.Module(),
	)

	app.Run()
}
