package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/katshuuu/pr-pigeon/internal/app"
	"github.com/katshuuu/pr-pigeon/internal/config"
	"github.com/katshuuu/pr-pigeon/internal/logger"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid configuration:\n%v\n", err)
		return 2
	}

	log := logger.New(cfg.Log, os.Stdout)
	log.Info("starting pr-pigeon", "version", version, "log_level", cfg.Log.Level.String())

	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("pr-pigeon stopped with error", "err", err)
		return 1
	}

	log.Info("pr-pigeon stopped, bye 👋")
	return 0
}
