package journal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "run-1.json")
	j := &Journal{
		RunID:        "run-1",
		State:        StateCommitting,
		StartedAt:    time.Now().UTC(),
		AppliedIndex: -1,
		Ops:          []Operation{{Kind: OperationUpsert, Path: "/etc/app", SourcePath: "/tmp/src", NodeType: NodeFile}},
		BackupRefs:   map[string]BackupRef{"/etc/app": {Exists: true, Path: filepath.Join(tmp, "bkp")}},
	}

	if err := Save(path, j); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.RunID != j.RunID || loaded.State != j.State || len(loaded.Ops) != 1 {
		t.Fatalf("loaded journal mismatch: %#v", loaded)
	}
}

func TestListPending(t *testing.T) {
	tmp := t.TempDir()
	if err := Save(filepath.Join(tmp, "one.json"), &Journal{RunID: "one", State: StateCommitting}); err != nil {
		t.Fatalf("save one: %v", err)
	}
	if err := Save(filepath.Join(tmp, "two.json"), &Journal{RunID: "two", State: StateCommitted}); err != nil {
		t.Fatalf("save two: %v", err)
	}

	pending, err := ListPending(tmp)
	if err != nil {
		t.Fatalf("ListPending returned error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending journal, got %d", len(pending))
	}
}
