package diff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

func TestScanUpperDir(t *testing.T) {
	tmp := t.TempDir()
	upper := filepath.Join(tmp, "upper")
	if err := os.MkdirAll(filepath.Join(upper, "etc"), 0o755); err != nil {
		t.Fatalf("mkdir upper: %v", err)
	}
	if err := os.WriteFile(filepath.Join(upper, "etc", "new.conf"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(upper, "etc", ".wh.old.conf"), []byte{}, 0o644); err != nil {
		t.Fatalf("write whiteout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(upper, "etc", ".wh..wh..opq"), []byte{}, 0o644); err != nil {
		t.Fatalf("write opaque marker: %v", err)
	}

	ops, err := ScanUpperDir(upper, "/")
	if err != nil {
		t.Fatalf("ScanUpperDir returned error: %v", err)
	}

	byPath := make(map[string]journal.Operation)
	for _, op := range ops {
		byPath[op.Path] = op
	}

	if _, ok := byPath["/etc/new.conf"]; !ok {
		t.Fatalf("missing upsert op for /etc/new.conf")
	}
	if op, ok := byPath["/etc/old.conf"]; !ok || op.Kind != journal.OperationDelete {
		t.Fatalf("missing delete op for /etc/old.conf")
	}
	if op, ok := byPath["/etc"]; !ok || !op.Opaque {
		t.Fatalf("expected /etc to be marked opaque")
	}
}

func TestPlanDeterministicOrder(t *testing.T) {
	ops := []journal.Operation{
		{Kind: journal.OperationDelete, Path: "/a/b/c"},
		{Kind: journal.OperationUpsert, Path: "/a", SourcePath: "/tmp/src-a", NodeType: journal.NodeDirectory},
		{Kind: journal.OperationUpsert, Path: "/a/b/file", SourcePath: "/tmp/src-f", NodeType: journal.NodeFile},
		{Kind: journal.OperationDelete, Path: "/a/b"},
	}

	ordered := Plan(ops)
	if len(ordered) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(ordered))
	}
	if ordered[0].Path != "/a" {
		t.Fatalf("expected directory upsert first, got %s", ordered[0].Path)
	}
	if ordered[3].Path != "/a/b" {
		t.Fatalf("expected shallowest delete last, got %s", ordered[3].Path)
	}
}
