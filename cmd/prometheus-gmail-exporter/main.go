package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/auth"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/server"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/updater"
	log "github.com/sirupsen/logrus"
)

var version = "dev"

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    false,
		DisableTimestamp: true,
	})
	log.SetLevel(log.InfoLevel)
	log.Info("prometheus-gmail-exporter is starting up.")

	ready := readiness.New()
	ready.Set("MAIN")

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	setLogLevel(cfg.LogLevel)
	logVersion()
	log.Infof("args (from config, and flags): %s", cfg)
	log.Infof("UID: %d", os.Getuid())
	log.Infof("Home directory: %s", os.Getenv("HOME"))

	_ = config.EnsureConfigDir()

	authMgr := auth.NewManager(cfg, ready)
	authMgr.TryMarkComplete()

	metricReg := metrics.NewRegistry()
	srv, err := server.New(cfg, authMgr, metricReg, ready)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	up := updater.New(cfg, authMgr, metricReg, ready)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Daemonize {
		runDaemon(ctx, up, cfg.UpdateDelaySeconds)
		return
	}

	if err := up.UpdateOnce(ctx); err != nil {
		log.Errorf("update: %v", err)
	}

	<-ctx.Done()
	log.Info("Ctrl+C, bye!")
}

func runDaemon(ctx context.Context, up *updater.Updater, delaySeconds int) {
	for {
		if err := up.UpdateOnce(ctx); err != nil {
			log.Errorf("update: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Info("Ctrl+C, bye!")
			return
		case <-time.After(time.Duration(delaySeconds) * time.Second):
		}
	}
}

func setLogLevel(level int) {
	switch {
	case level <= 10:
		log.SetLevel(log.DebugLevel)
	case level <= 20:
		log.SetLevel(log.InfoLevel)
	case level <= 30:
		log.SetLevel(log.WarnLevel)
	default:
		log.SetLevel(log.ErrorLevel)
	}
}

func logVersion() {
	log.Infof("Version: %s", version)
	if data, err := os.ReadFile("VERSION"); err == nil {
		log.Infof("Build: %s", string(data))
	}
}
