package testutil

import "testing"

// TestStubConstructsAndCloses keeps the package testable so `go test -cover ./...`
// is happy in CI — Go 1.24's coverage tool errors on pure-helper packages that
// compile with _test.go but contain no test files.
func TestStubConstructsAndCloses(t *testing.T) {
	s := NewStub("secret")
	defer s.Close()
	if s.AuthKey != "secret" {
		t.Errorf("AuthKey = %q, want secret", s.AuthKey)
	}
	if s.BaseURL() == "" {
		t.Error("BaseURL is empty")
	}
	if s.ListenerAddr() == nil {
		t.Error("ListenerAddr is nil")
	}
	if len(s.Log) != 0 {
		t.Errorf("Log should start empty, got %d entries", len(s.Log))
	}
}
