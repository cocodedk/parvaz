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

func ptrBool(b bool) *bool { return &b }

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
		InsecureTLS: ptrBool(true),
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
	if !got.InsecureTLSEnabled() {
		t.Errorf("InsecureTLS not merged (stdin set to true)")
	}
	// Flag defaults preserved when stdin is silent
	if got.FrontDomain != defaultFrontDomain {
		t.Errorf("FrontDomain lost: %q", got.FrontDomain)
	}
	if got.ListenPort != defaultListenPort {
		t.Errorf("ListenPort lost: %d", got.ListenPort)
	}
}

func TestMerge_InsecureTLS_StdinWinsOnOverlap(t *testing.T) {
	// stdin explicitly false MUST override a flag-supplied true — the
	// documented "stdin wins on overlap" contract. Pointer type makes
	// this possible (nil vs. *false are distinguishable).
	base := Config{InsecureTLS: ptrBool(true)}
	stdin := Config{InsecureTLS: ptrBool(false)}
	if got := merge(base, stdin); got.InsecureTLSEnabled() {
		t.Error("stdin=false must override base=true; got InsecureTLS stayed true")
	}
}

func TestMerge_InsecureTLS_StdinNilLeavesBaseAlone(t *testing.T) {
	// Absent from stdin JSON ({} or no insecure_tls key) unmarshals to
	// nil, which leaves base untouched.
	base := Config{InsecureTLS: ptrBool(true)}
	stdin := Config{}
	if got := merge(base, stdin); !got.InsecureTLSEnabled() {
		t.Error("stdin nil clobbered base=true; must leave base alone")
	}
}

func TestMerge_FrontPort_ZeroInStdinLeavesBaseAlone(t *testing.T) {
	base := Config{FrontPort: 443}
	stdin := Config{} // no front_port set
	if got := merge(base, stdin); got.FrontPort != 443 {
		t.Errorf("zero FrontPort in stdin clobbered base: %d", got.FrontPort)
	}
}
