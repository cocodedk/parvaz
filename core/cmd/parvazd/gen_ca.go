package main

import (
	"errors"
	"fmt"

	"github.com/cocodedk/parvaz/core/mitm"
)

// genCA materialises the MITM root CA in dataDir/ca/ via mitm.LoadOrCreate.
// Idempotent — subsequent calls reuse the existing CA so the fingerprint
// the Android app already installed stays valid.
//
// The flow: `parvazd -gen-ca -data-dir <app-files>/parvaz-data` runs to
// completion before the Kotlin CaInstallScreen reads `ca.crt` and hands
// it to ACTION_MANAGE_CA_CERTIFICATES. Keeping this separate from the
// full sidecar path avoids binding :1080 + spinning up the dispatcher
// just to write one file.
func genCA(dataDir string) error {
	if dataDir == "" {
		return errors.New("gen-ca: data_dir required")
	}
	if _, err := mitm.LoadOrCreate(dataDir); err != nil {
		return fmt.Errorf("gen-ca: %w", err)
	}
	return nil
}
