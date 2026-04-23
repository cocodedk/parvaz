package dispatcher_test

import (
	"testing"

	"github.com/cocodedk/parvaz/core/dispatcher"
	"github.com/cocodedk/parvaz/core/mitm"
)

// Verifies *mitm.Interceptor and *mitm.SNITunnel satisfy the dispatcher
// interfaces structurally. The assignments are the check; bodies empty.

func TestInterceptorCompat(t *testing.T) {
	var _ dispatcher.Interceptor = (*mitm.Interceptor)(nil)
}

func TestSNITunnelCompat(t *testing.T) {
	var _ dispatcher.SNITunneler = (*mitm.SNITunnel)(nil)
}
