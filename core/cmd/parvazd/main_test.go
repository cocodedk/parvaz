package main

import "testing"

func TestConfig_Validate(t *testing.T) {
	ok := Config{
		AuthKey:    "k",
		ScriptURLs: []string{"https://x/exec"},
		DataDir:    "/tmp/parvaz-data",
	}
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty", Config{}, true},
		{"auth only", Config{AuthKey: "k"}, true},
		{"scripts only", Config{ScriptURLs: []string{"https://x/exec"}}, true},
		{"no data_dir", Config{AuthKey: "k", ScriptURLs: []string{"https://x/exec"}}, true},
		{"complete", ok, false},
	}
	for _, tc := range cases {
		err := tc.cfg.validate()
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

func TestMerge_StdinOverridesFlagDefaults(t *testing.T) {
	base := Config{
		GoogleIP: defaultGoogleIP, FrontDomain: defaultFrontDomain,
		ListenHost: defaultListenHost, ListenPort: defaultListenPort,
		FrontPort: defaultFrontPort,
	}
	stdin := Config{
		ScriptURLs:  []string{"https://script.google.com/macros/s/ABC/exec"},
		AuthKey:     "secret-from-stdin",
		GoogleIP:    "64.233.160.0",
		DataDir:     "/var/lib/parvaz",
		FrontPort:   8443,
		InsecureTLS: true,
	}
	got := merge(base, stdin)
	if got.DataDir != "/var/lib/parvaz" {
		t.Errorf("DataDir not merged: %q", got.DataDir)
	}
	if len(got.ScriptURLs) != 1 || got.ScriptURLs[0] != stdin.ScriptURLs[0] {
		t.Errorf("ScriptURLs = %v, want %v", got.ScriptURLs, stdin.ScriptURLs)
	}
	if got.AuthKey != stdin.AuthKey {
		t.Errorf("AuthKey not merged")
	}
	if got.GoogleIP != "64.233.160.0" {
		t.Errorf("GoogleIP = %q, want 64.233.160.0", got.GoogleIP)
	}
	if got.FrontPort != 8443 {
		t.Errorf("FrontPort not merged: %d, want 8443", got.FrontPort)
	}
	if !got.InsecureTLS {
		t.Errorf("InsecureTLS not merged")
	}
	// Flag defaults preserved when stdin is silent
	if got.FrontDomain != defaultFrontDomain {
		t.Errorf("FrontDomain lost: %q", got.FrontDomain)
	}
	if got.ListenPort != defaultListenPort {
		t.Errorf("ListenPort lost: %d", got.ListenPort)
	}
}

func TestMerge_InsecureTLS_StdinFalseDoesNotClobberBaseTrue(t *testing.T) {
	// Edge case: a zero-value bool in stdin must not silently flip a
	// base "true" back to "false". Ops cheatsheet: stdin wins only when
	// it sets the field explicitly; we can't distinguish "unset" from
	// "explicit false" on a bool without a *bool, so stdin "false" is
	// treated as unset. (Symmetric with the other Config fields where
	// zero values leave base alone.)
	base := Config{InsecureTLS: true}
	stdin := Config{InsecureTLS: false}
	if got := merge(base, stdin); !got.InsecureTLS {
		t.Error("stdin=false clobbered base=true for InsecureTLS; must treat false as unset")
	}
}

func TestMerge_FrontPort_ZeroInStdinLeavesBaseAlone(t *testing.T) {
	base := Config{FrontPort: 443}
	stdin := Config{} // no front_port set
	if got := merge(base, stdin); got.FrontPort != 443 {
		t.Errorf("zero FrontPort in stdin clobbered base: %d", got.FrontPort)
	}
}
