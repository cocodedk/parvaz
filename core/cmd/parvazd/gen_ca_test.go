package main

import (
	"crypto/sha256"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// genCA is the entry point for `parvazd -gen-ca`. The Android app runs
// it once before the CA-install screen to materialise the PEM, then
// reads the PEM and hands it to ACTION_MANAGE_CA_CERTIFICATES.

func TestGenCA_CreatesPersistedPEM(t *testing.T) {
	dir := t.TempDir()
	if err := genCA(dir); err != nil {
		t.Fatalf("genCA: %v", err)
	}
	crtPath := filepath.Join(dir, "ca", "ca.crt")
	keyPath := filepath.Join(dir, "ca", "ca.key")

	crtBytes, err := os.ReadFile(crtPath)
	if err != nil {
		t.Fatalf("read ca.crt: %v", err)
	}
	block, _ := pem.Decode(crtBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("ca.crt not a PEM CERTIFICATE block")
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat ca.key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("ca.key perm = %v, want 0600", info.Mode().Perm())
	}
}

func TestGenCA_IdempotentPreservesExistingCA(t *testing.T) {
	dir := t.TempDir()
	if err := genCA(dir); err != nil {
		t.Fatalf("first gen: %v", err)
	}
	crtPath := filepath.Join(dir, "ca", "ca.crt")
	first, err := os.ReadFile(crtPath)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	firstSum := sha256.Sum256(first)

	if err := genCA(dir); err != nil {
		t.Fatalf("second gen: %v", err)
	}
	second, err := os.ReadFile(crtPath)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	secondSum := sha256.Sum256(second)

	if firstSum != secondSum {
		t.Error("CA regenerated on second run — must be idempotent (Android relies on a stable fingerprint after install)")
	}
}

func TestGenCA_EmptyDataDirRejected(t *testing.T) {
	if err := genCA(""); err == nil {
		t.Error("genCA(\"\") should return an error; got nil")
	}
}
