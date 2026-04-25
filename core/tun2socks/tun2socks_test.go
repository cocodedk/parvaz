package tun2socks

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestStart_RejectsZeroFD(t *testing.T) {
	r := NewRunner(newSilentLogger())
	err := r.Start(Config{FD: 0, MTU: 1500, SOCKS5Addr: "127.0.0.1:1080"})
	if err == nil || !strings.Contains(err.Error(), "FD must be > 0") {
		t.Errorf("expected FD-validation error, got %v", err)
	}
}

func TestStart_RejectsMissingSOCKS5Addr(t *testing.T) {
	r := NewRunner(newSilentLogger())
	err := r.Start(Config{FD: 3, MTU: 1500})
	if err == nil || !strings.Contains(err.Error(), "SOCKS5Addr") {
		t.Errorf("expected SOCKS5Addr-validation error, got %v", err)
	}
}

func TestStart_RejectsDuplicate(t *testing.T) {
	// Validation errors don't burn the started flag, so we have to
	// arrange a successful CAS first. The engine.Insert/Start pair
	// would block on a real TUN, so this test only covers the gate
	// itself by faking a previously-started runner.
	r := NewRunner(newSilentLogger())
	r.started.Store(true)
	err := r.Start(Config{FD: 3, MTU: 1500, SOCKS5Addr: "127.0.0.1:1080"})
	if err != ErrAlreadyStarted {
		t.Errorf("expected ErrAlreadyStarted, got %v", err)
	}
}

// The real engine start needs a Linux TUN device and Android packet
// plumbing — not realistic in a JVM-free host unit test. Integration
// coverage lives in scripts/e2e and the emulator walkthrough.
