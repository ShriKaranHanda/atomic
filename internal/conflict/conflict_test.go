package conflict

import (
	"testing"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

func TestDetectConflictOnChangedPath(t *testing.T) {
	txnStart := time.Unix(100, 0)
	ops := []journal.Operation{{Path: "/etc/app.conf"}}

	statFn := func(path string) (FileState, error) {
		return FileState{Exists: true, CTime: time.Unix(120, 0)}, nil
	}

	err := Check(ops, txnStart, statFn)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestNoConflictWhenUnchanged(t *testing.T) {
	txnStart := time.Unix(100, 0)
	ops := []journal.Operation{{Path: "/etc/app.conf"}}

	statFn := func(path string) (FileState, error) {
		return FileState{Exists: true, CTime: time.Unix(90, 0)}, nil
	}

	if err := Check(ops, txnStart, statFn); err != nil {
		t.Fatalf("unexpected conflict error: %v", err)
	}
}
