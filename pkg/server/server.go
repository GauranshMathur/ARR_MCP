// Package server exposes an *arr media stack over the Model Context Protocol.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/GauranshMathur/ARR_MCP/pkg/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wires configured service instances into an MCP server.
type Server struct {
	cfg *config.Config
	log *logger.Logger
	mcp *mcp.Server
}

// New builds a server exposing tools for every configured service instance.
func New(cfg *config.Config, log *logger.Logger) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
		mcp: mcp.NewServer(&mcp.Implementation{Name: "arr-mcp", Version: Version}, nil),
	}
	registerAll(s)
	return s
}

// MCP returns the underlying protocol server, for transports and tests.
func (s *Server) MCP() *mcp.Server { return s.mcp }

// gateFor returns the permission gate governing a specific instance, honouring
// any per-instance override of the global policy.
func (s *Server) gateFor(inst *config.Instance) Gate {
	return Gate{Perms: s.cfg.EffectivePermissions(inst)}
}

// registersForService reports whether any configured instance of a service
// permits tools at this access tier. A single unrestricted instance is enough
// to justify advertising the tool; per-instance policy is enforced at call time.
func (s *Server) registersForService(service string, a Access) bool {
	instances := s.cfg.Services[service]
	if len(instances) == 0 {
		return false
	}
	for i := range instances {
		if s.gateFor(&instances[i]).Registers(a) {
			return true
		}
	}
	return false
}

// RunStdio serves MCP over stdin/stdout until the client disconnects. Nothing
// else may write to stdout in this mode: it carries the JSON-RPC stream.
func (s *Server) RunStdio(ctx context.Context) error {
	s.log.Info("serving MCP over stdio")
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

// RunHTTP serves MCP over Streamable HTTP at /mcp, plus a /health probe.
func (s *Server) RunHTTP(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.mcp }, nil))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// A fresh context is required below, not a child of ctx: this goroutine only
	// runs because ctx was cancelled, and a cancelled parent would make Shutdown
	// return immediately instead of draining in-flight requests.
	go func() { // #nosec G118
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.log.Info("serving MCP over HTTP on %s (endpoint /mcp)", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}
