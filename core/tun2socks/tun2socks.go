// Package tun2socks bridges Android's TUN file descriptor into the
// parvazd dispatcher. The Kotlin VpnService establishes the TUN and
// passes the raw fd (with FD_CLOEXEC cleared) through stdin JSON; we
// open it with xjasonlyu/tun2socks/v2 and forward every TCP flow +
// UDP packet via the sidecar's loopback SOCKS5 server. That server's
// dialer is parvazd's dispatcher, so the existing MITM / SNI-rewrite /
// Apps Script routing applies without code changes.
//
// Per codex's architecture review: loopback SOCKS5 adds one in-process
// hop but zero correctness risk, vs. a custom engine dialer that'd
// duplicate the dispatcher plumbing. Optimise away the hop later.
package tun2socks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

// ErrAlreadyStarted is returned by Runner.Start on a second call.
// The xjasonlyu engine is a process-global singleton — silently
// replacing the running config would mask a real wiring bug.
var ErrAlreadyStarted = errors.New("tun2socks: runner already started")

// Config holds the wiring for a running tun2socks instance.
type Config struct {
	// FD is the raw Android TUN file descriptor. Must be > 0.
	FD int
	// MTU must match VpnService.Builder.setMtu on the Kotlin side.
	MTU int
	// SOCKS5Addr is the loopback SOCKS5 server parvazd listens on
	// (typically 127.0.0.1:1080). All TCP flows tun2socks sees are
	// relayed through it; the server's dialer is our dispatcher.
	SOCKS5Addr string
	// LogLevel forwards to tun2socks' zap logger. "warn" or "info"
	// for production; "debug" to troubleshoot packet parsing.
	LogLevel string
}

// Runner wraps the xjasonlyu engine's insert/start/stop so the rest
// of parvazd doesn't have to talk to the global singleton directly.
// Only one Runner per process — engine.Insert replaces the current
// configuration, it isn't additive. The started flag turns a misuse
// into a loud error rather than a silent reconfigure.
type Runner struct {
	logger  *slog.Logger
	started atomic.Bool
}

func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{logger: logger}
}

// Start configures the engine and spins it up on its own goroutines.
// Returns once the engine is running — caller should print READY AFTER
// this returns so Kotlin doesn't flip the UI to CONNECTED before the
// data plane is live.
//
// engine.Start/Stop from xjasonlyu return nothing: internally they
// log.Fatalf on failure, which exits the process. CoreLauncher.start
// on the Android side sees EOF on stdout before READY and reports
// failure — so the "READY after Start" ordering gives Kotlin a
// truthful signal without us having to add more error paths.
func (r *Runner) Start(cfg Config) error {
	if cfg.FD <= 0 {
		return fmt.Errorf("tun2socks: FD must be > 0, got %d", cfg.FD)
	}
	if cfg.MTU <= 0 {
		cfg.MTU = 1500
	}
	if cfg.SOCKS5Addr == "" {
		return fmt.Errorf("tun2socks: SOCKS5Addr required")
	}
	// Loud-fail on a duplicate Start: the xjasonlyu engine is a
	// process-global singleton, so a second Insert+Start would swap
	// the live config out from under whoever's already using it.
	if !r.started.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "warn"
	}

	key := &engine.Key{
		Device:   fmt.Sprintf("fd://%d", cfg.FD),
		Proxy:    "socks5://" + cfg.SOCKS5Addr,
		MTU:      cfg.MTU,
		LogLevel: logLevel,
	}
	engine.Insert(key)
	r.logger.Info("tun2socks: inserted engine key",
		"fd", cfg.FD, "mtu", cfg.MTU, "proxy", key.Proxy)

	engine.Start()
	r.logger.Info("tun2socks: engine running")
	return nil
}

// Wait blocks until [ctx] is cancelled, then stops the engine. Invoke
// in a goroutine after [Start] returns; separates the lifecycle from
// the startup handshake so callers can print READY in between.
func (r *Runner) Wait(ctx context.Context) {
	<-ctx.Done()
	r.Stop()
}

// Stop halts the engine. Safe to call multiple times; the xjasonlyu
// engine treats a duplicate stop as no-op.
func (r *Runner) Stop() {
	engine.Stop()
	r.logger.Info("tun2socks: engine stopped")
}
