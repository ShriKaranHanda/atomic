package commit

import (
	"path/filepath"
	"testing"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

func TestApplyUpsert(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	srcRoot := filepath.Join(tmp, "src")
	if err := ensureDir(root); err != nil {
		t.Fatalf("ensure root: %v", err)
	}
	if err := ensureDir(srcRoot); err != nil {
		t.Fatalf("ensure srcRoot: %v", err)
	}
	src := filepath.Join(srcRoot, "app.conf")
	if err := writeFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	eng := Engine{RootPrefix: root}
	j := &journal.Journal{
		RunID:        "run-1",
		State:        journal.StateCommitting,
		AppliedIndex: -1,
		Ops: []journal.Operation{{
			Kind:       journal.OperationUpsert,
			Path:       "/etc/app.conf",
			SourcePath: src,
			NodeType:   journal.NodeFile,
		}},
		BackupRefs: map[string]journal.BackupRef{},
		BackupDir:  filepath.Join(tmp, "backups"),
	}
	journalPath := filepath.Join(tmp, "run-1.json")
	if err := eng.Apply(journalPath, j); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	got, err := readFile(filepath.Join(root, "etc", "app.conf"))
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("unexpected committed content %q", got)
	}
}

func TestRollbackOnFailure(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := ensureDir(filepath.Join(root, "etc")); err != nil {
		t.Fatalf("ensure etc: %v", err)
	}
	target := filepath.Join(root, "etc", "app.conf")
	if err := writeFile(target, []byte("original"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	goodSrc := filepath.Join(tmp, "good")
	if err := writeFile(goodSrc, []byte("new"), 0o644); err != nil {
		t.Fatalf("write good src: %v", err)
	}

	eng := Engine{RootPrefix: root}
	j := &journal.Journal{
		RunID:        "run-2",
		State:        journal.StateCommitting,
		AppliedIndex: -1,
		Ops: []journal.Operation{
			{Kind: journal.OperationUpsert, Path: "/etc/app.conf", SourcePath: goodSrc, NodeType: journal.NodeFile},
			{Kind: journal.OperationUpsert, Path: "/etc/missing.conf", SourcePath: filepath.Join(tmp, "missing"), NodeType: journal.NodeFile},
		},
		BackupRefs: map[string]journal.BackupRef{},
		BackupDir:  filepath.Join(tmp, "backups"),
	}
	journalPath := filepath.Join(tmp, "run-2.json")
	if err := eng.Apply(journalPath, j); err == nil {
		t.Fatalf("expected apply failure")
	}

	got, err := readFile(target)
	if err != nil {
		t.Fatalf("read rolled-back file: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("rollback failed, expected original, got %q", got)
	}
}
