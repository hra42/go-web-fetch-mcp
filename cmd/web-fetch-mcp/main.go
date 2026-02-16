package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
	"github.com/hra42/go-web-fetch-mcp/internal/processor"
	"github.com/hra42/go-web-fetch-mcp/internal/robots"
	"github.com/hra42/go-web-fetch-mcp/internal/server"
)

func main() {
	log.SetOutput(os.Stderr)

	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.UserAgent, "user-agent", cfg.UserAgent, "User-Agent header for HTTP requests")
	flag.BoolVar(&cfg.IgnoreRobotsTxt, "ignore-robots-txt", cfg.IgnoreRobotsTxt, "Skip robots.txt checks")
	flag.StringVar(&cfg.ProxyURL, "proxy-url", cfg.ProxyURL, "HTTP/HTTPS proxy URL")
	flag.Parse()

	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := server.NewServer(cfg, f, p)

	if !cfg.IgnoreRobotsTxt {
		srv.SetRobotsChecker(robots.NewChecker(f))
	}

	if err := srv.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
