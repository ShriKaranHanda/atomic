package cli

import (
	"testing"

	"github.com/ShriKaranHanda/atomic/internal/exitcode"
	"github.com/ShriKaranHanda/atomic/internal/ipc"
)

func TestResultMessageScriptFailureIsRedRollbackMessage(t *testing.T) {
	ev := ipc.Event{AtomicExitCode: exitcode.ScriptFailed, Message: "script failed"}
	got := resultMessage(ev)
	want := "\x1b[31matomic: script failed. Reverting filesystem changes.\x1b[0m"
	if got != want {
		t.Fatalf("unexpected script failure message: got %q want %q", got, want)
	}
}

func TestResultMessageUsesDaemonMessageForNonScriptFailure(t *testing.T) {
	ev := ipc.Event{AtomicExitCode: exitcode.Conflict, Message: "conflict detected"}
	got := resultMessage(ev)
	if got != "conflict detected" {
		t.Fatalf("unexpected non-script message: got %q", got)
	}
}
