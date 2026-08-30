// Command arr-mcp serves an *arr media stack over the Model Context Protocol.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/GauranshMathur/ARR_MCP/pkg/logger"
	"github.com/GauranshMathur/ARR_MCP/pkg/server"
)

// specs maps a service name to the API description used to reach it.
var specs = map[string]arr.ServiceSpec{
	"sonarr":      arr.SonarrSpec,
	"radarr":      arr.RadarrSpec,
	"prowlarr":    arr.ProwlarrSpec,
	"bazarr":      arr.BazarrSpec,
	"qbittorrent": arr.QBittorrentSpec,
	"nzbget":      arr.NZBGetSpec,
}

func main() {
	var (
		configPath = flag.String("config", os.Getenv("ARR_MCP_CONFIG"), "path to config.yaml; omit to configure from environment variables")
		transport  = flag.String("transport", "", "transport to serve: stdio or http (overrides config)")
		addr       = flag.String("addr", "", "listen address for the http transport (overrides config)")
		logLevel   = flag.String("log-level", "", "log level: debug, info, warn, error (overrides config)")
		check      = flag.Bool("check", false, "check connectivity to every configured instance and exit")
		showVer    = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("arr-mcp", server.Version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Flags win over the config file so a container can be retargeted without
	// rewriting its mounted configuration.
	if *transport != "" {
		cfg.Server.Transport = *transport
	}
	if *addr != "" {
		cfg.Server.Addr = *addr
	}
	if *logLevel != "" {
		cfg.Server.LogLevel = *logLevel
	}

	// Logging goes to stderr: under stdio, stdout carries the JSON-RPC stream.
	log := logger.New(cfg.Server.LogLevel, "arr-mcp")

	if *check {
		os.Exit(checkAll(cfg, log))
	}

	for _, svc := range cfg.ConfiguredServices() {
		log.Info("%s: %d instance(s) configured [%s]", svc, len(cfg.Services[svc]),
			strings.Join(cfg.InstanceNames(svc), ", "))
	}
	log.Info("permissions: mode=%s confirmScope=%s fallback=%s",
		cfg.Permissions.Mode, cfg.Permissions.ConfirmScope, cfg.Permissions.Fallback)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := server.New(cfg, log)

	switch cfg.Server.Transport {
	case "stdio":
		err = s.RunStdio(ctx)
	case "http":
		err = s.RunHTTP(ctx, cfg.Server.Addr)
	default:
		fmt.Fprintf(os.Stderr, "unknown transport %q; want stdio or http\n", cfg.Server.Transport)
		os.Exit(1)
	}

	if err != nil && ctx.Err() == nil {
		log.Error("server stopped: %v", err)
		os.Exit(1)
	}
	log.Info("shutdown complete")
}

// checkAll pings every configured instance in parallel and reports the results.
// It returns a process exit code.
func checkAll(cfg *config.Config, log *logger.Logger) int {
	ctx := context.Background()
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		failed bool
	)

	for _, svc := range cfg.ConfiguredServices() {
		spec, ok := specs[svc]
		if !ok {
			continue
		}
		for i := range cfg.Services[svc] {
			inst := cfg.Services[svc][i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := arr.NewClient(inst.URL, spec, server.InstanceCredentials(&inst))
				err := client.Ping(ctx)

				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failed = true
					fmt.Printf("FAIL  %s/%s (%s): %v\n", svc, inst.Name, inst.URL, err)
					return
				}
				fmt.Printf("OK    %s/%s (%s)\n", svc, inst.Name, inst.URL)
			}()
		}
	}
	wg.Wait()

	if failed {
		return 1
	}
	return 0
}
