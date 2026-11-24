package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifiedTLSConfig(t *testing.T) {
	c := Config{URL: "mysql://demo:secret@example.com:3306/test?ssl-mode=REQUIRED", CAFile: "certs/aiven-ca.pem"}
	cfg, e := driverConfig(c)
	if e != nil {
		t.Fatal(e)
	}
	if cfg.TLSConfig == "" || cfg.TLSConfig == "skip-verify" || cfg.User != "demo" || cfg.Passwd != "secret" || cfg.DBName != "test" {
		t.Fatal("incorrect connection config")
	}
}
func TestRejectInvalidConfig(t *testing.T) {
	for _, c := range []Config{{URL: "https://example.com"}, {URL: "mysql://demo:x@example.com:3306/test"}} {
		if _, e := driverConfig(c); e == nil {
			t.Fatal("accepted invalid config")
		}
	}
	p := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(p, []byte("invalid"), 0600)
	if _, e := driverConfig(Config{URL: "mysql://u:p@example.com:3306/db", CAFile: p}); e == nil {
		t.Fatal("accepted invalid CA")
	}
}
