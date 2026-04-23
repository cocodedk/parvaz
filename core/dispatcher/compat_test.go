package dispatcher_test

import (
	"testing"

	"github.com/cocodedk/parvaz/core/dispatcher"
	"github.com/cocodedk/parvaz/core/mitm"
)

// Verifies *mitm.Interceptor satisfies dispatcher.Interceptor structurally.
// The assignment is the check; the test body is empty.
func TestInterceptorCompat(t *testing.T) {
	var _ dispatcher.Interceptor = (*mitm.Interceptor)(nil)
}
