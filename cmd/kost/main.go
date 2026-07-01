package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nirjxr26/Kost/internal/analyze"
	"github.com/nirjxr26/Kost/internal/config"
	"github.com/nirjxr26/Kost/internal/k8s"
	"github.com/nirjxr26/Kost/internal/report"
)

const (
	collectTimeout = 30 * time.Second
	shutdownGrace  = 5 * time.Second
)

func main() {
	cfg := loadConfig()
	client := mustK8sClient()
	an := analyze.New(cfg.ClusterName, cfg.WasteRatio, cfg.MinWaste, cfg.CPUPerCore, cfg.MemPerGB)
	latest := report.NewLatest()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler(client))
	mux.HandleFunc("/metrics", report.MetricsHandler(cfg.ClusterName, latest))

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: mux}
	go func() {
		log.Printf("HTTP server on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server: %v", err)
		}
	}()

	interval, _ := cfg.ParseInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("kost started cluster=%q interval=%s", cfg.ClusterName, interval)

	runAndReport(ctx, client, an, latest)
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
			defer cancel()
			srv.Shutdown(shutdownCtx)
			log.Println("shutdown complete")
			return
		case <-ticker.C:
			runAndReport(ctx, client, an, latest)
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// readyHandler returns 200 only when the K8s client can list pods.
func readyHandler(client *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, err := client.ListPods(ctx)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "not ready: %v\n", err)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ready")
	}
}

func loadConfig() *config.Config {
	configPath := flag.String("config", "/etc/kost/config.json", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if u := os.Getenv("SLACK_WEBHOOK_URL"); u != "" {
		_ = u // ponytail: Slack reporter deferred
	}
	return cfg
}

func mustK8sClient() *k8s.Client {
	client, err := k8s.InCluster()
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}
	return client
}

func runAndReport(ctx context.Context, client *k8s.Client, an *analyze.Analyzer, latest *report.LatestReport) {
	ctx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	usages, err := client.PodUsages(ctx)
	if err != nil {
		log.Printf("collect: %v", err)
		return
	}
	r := an.Run(usages)
	latest.Store(r)
	if err := report.Stdout(r); err != nil {
		log.Printf("report: %v", err)
	}
	report.LogReport(r)
}
