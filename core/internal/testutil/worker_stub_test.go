package testutil

import "testing"

// TestWorkerStub_LifecycleAndFields keeps the package testable so
// `go test -cover ./...` is happy in Go 1.24.
func TestWorkerStub_LifecycleAndFields(t *testing.T) {
	s := NewWorkerStub("k")
	defer s.Close()
	if s.AuthKey != "k" {
		t.Errorf("AuthKey = %q", s.AuthKey)
	}
	if s.HTTPClient() == nil {
		t.Error("HTTPClient nil")
	}
	if s.WSURL() == "" {
		t.Error("WSURL empty")
	}
	if s.Hits() != 0 {
		t.Errorf("Hits = %d, want 0", s.Hits())
	}
}
