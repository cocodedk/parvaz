package main

import "testing"

// Compile-time smoke test so `go test ./...` doesn't fail with
// "no test files" in this package. Real coverage lives in the
// scripts/e2e harness + testutil.AppsScriptStub's own tests.
func TestStubCompiles(t *testing.T) {
	_ = run
	_ = selfSignedCert
}
