package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/controller"
	"github.com/misaf/vendra-controller/internal/docker"
	"github.com/misaf/vendra-controller/internal/process"
	"github.com/misaf/vendra-controller/internal/provisioner"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		cfg, err := config.Load("")
		if err == nil {
			err = provisioner.Healthcheck(cfg.Listen)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	cfg, err := config.Load("")
	if err != nil {
		fatal(err)
	}
	if cfg.ProvisionerToken == "" {
		fatal(fmt.Errorf("VENDRA_PROVISIONER_TOKEN is required"))
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	c := controller.New(cfg, docker.Service{Runner: process.ExecRunner{}})
	server := provisioner.HTTPServer(cfg.Listen, provisioner.New(c, cfg.ProvisionerToken, version, logger).Handler())
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); _ = server.Shutdown(context.Background()) }()
	logger.Info("provisioner listening", "address", cfg.Listen, "version", version)
	if err := server.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
