// SPDX-License-Identifier: GPL-3.0-only
package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestImportSingleURI(t *testing.T) {
	servers, err := Import("vless://u@a.com:443?pbk=K#n")
	if err != nil || len(servers) != 1 {
		t.Fatalf("single uri import: %v %+v", err, servers)
	}
}

func TestImportRemoteURLSentinel(t *testing.T) {
	_, err := Import("https://example.com/sub")
	if !errors.Is(err, ErrRemote) {
		t.Fatalf("want ErrRemote, got %v", err)
	}
}

func TestImportFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub.txt")
	os.WriteFile(p, []byte("vless://u@a.com:443?pbk=K#n\n"), 0o600)
	servers, err := Import(p)
	if err != nil || len(servers) != 1 {
		t.Fatalf("file import: %v %+v", err, servers)
	}
}
