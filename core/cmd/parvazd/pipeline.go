package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/cocodedk/parvaz/core/dispatcher"
	"github.com/cocodedk/parvaz/core/mitm"
	"github.com/cocodedk/parvaz/core/relay"
	"github.com/cocodedk/parvaz/core/socks5"
)

// buildPipeline wires the full request path:
//
//	socks5.Server → dispatcher.Dispatcher → (allow-list)
//	                                       → direct TCP (Google hosts)
//	                                       → mitm.Interceptor → relay.Relay
//	                                         → fronter.HTTPClient → Apps Script
//
// Returns a socks5.Server ready to Serve. Everything it touches is
// refreshable on restart — the CA persists under cfg.DataDir, nothing
// else needs state.
func buildPipeline(cfg Config, logger *slog.Logger) (*socks5.Server, error) {
	client := buildHTTPClient(cfg)
	rel, err := relay.New(relay.Config{
		HTTPClient: client, ScriptURLs: cfg.ScriptURLs, AuthKey: cfg.AuthKey,
	})
	if err != nil {
		return nil, fmt.Errorf("relay: %w", err)
	}
	// Resolve DataDir to absolute so running parvazd from a different CWD
	// can't silently generate a second CA — Android's installed user-root
	// would no longer chain to it and every MITM would fail with
	// untrusted-cert errors.
	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data_dir: %w", err)
	}
	ca, err := mitm.LoadOrCreate(dataDir)
	if err != nil {
		return nil, fmt.Errorf("mitm ca: %w", err)
	}
	interceptor := &mitm.Interceptor{CA: ca, Relay: rel, Logger: logger}
	disp := &dispatcher.Dispatcher{
		AllowList:   dispatcher.DefaultAllowList,
		Interceptor: interceptor,
		Logger:      logger,
	}
	return &socks5.Server{Dialer: disp, Logger: logger}, nil
}
