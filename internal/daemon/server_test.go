package daemon

import "testing"

func TestSingleActiveTransactionLock(t *testing.T) {
	s := &Server{}
	if !s.acquireRun() {
		t.Fatalf("expected first acquire to succeed")
	}
	if s.acquireRun() {
		t.Fatalf("expected second acquire to fail while running")
	}
	s.releaseRun()
	if !s.acquireRun() {
		t.Fatalf("expected acquire after release to succeed")
	}
	s.releaseRun()
}
