package certs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCertificatesCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	caPath := filepath.Join(dir, "ca.crt")

	mgr := NewCertManager(certPath, keyPath, caPath)
	if err := mgr.EnsureCertificates(); err != nil {
		t.Fatalf("EnsureCertificates: %v", err)
	}

	for _, p := range []string{certPath, keyPath, caPath} {
		if info, err := os.Stat(p); err != nil || info.Size() == 0 {
			t.Fatalf("expected cert artifact %s to exist and be non-empty", p)
		}
	}
}
