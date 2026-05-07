package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
	"github.com/hra42/go-web-fetch-mcp/internal/processor"
	"github.com/hra42/go-web-fetch-mcp/internal/robots"
	"github.com/hra42/go-web-fetch-mcp/internal/server"
)

const authTokenEnv = "WEB_FETCH_MCP_TOKEN"

func main() {
	log.SetOutput(os.Stderr)

	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.UserAgent, "user-agent", cfg.UserAgent, "User-Agent header for HTTP requests")
	flag.BoolVar(&cfg.IgnoreRobotsTxt, "ignore-robots-txt", cfg.IgnoreRobotsTxt, "Skip robots.txt checks")
	flag.StringVar(&cfg.ProxyURL, "proxy-url", cfg.ProxyURL, "HTTP/HTTPS proxy URL")
	flag.StringVar(&cfg.Transport, "transport", cfg.Transport, "MCP transport: stdio or http")
	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "Bind address for http transport (e.g. :8080)")
	flag.Parse()

	cfg.AuthToken = os.Getenv(authTokenEnv)

	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := server.NewServer(cfg, f, p)

	if !cfg.IgnoreRobotsTxt {
		srv.SetRobotsChecker(robots.NewChecker(f))
	}

	switch cfg.Transport {
	case config.TransportStdio:
		if err := srv.Run(context.Background()); err != nil {
			log.Fatal(err)
		}
	case config.TransportHTTP:
		if cfg.AuthToken == "" {
			log.Fatalf("%s must be set when --transport=http", authTokenEnv)
		}
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := srv.RunHTTP(ctx, cfg.ListenAddr, cfg.AuthToken); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown transport %q (expected %q or %q)", cfg.Transport, config.TransportStdio, config.TransportHTTP)
	}
}
