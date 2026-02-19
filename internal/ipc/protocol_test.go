package ipc

import (
	"bytes"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)
	r := NewReader(buf)

	in := Request{Type: RequestRun, Version: Version, ScriptPath: "/tmp/a.sh", ScriptArgs: []string{"x"}, CWD: "/tmp", KeepArtifacts: true, Verbose: true}
	if err := w.WriteRequest(in); err != nil {
		t.Fatalf("WriteRequest returned error: %v", err)
	}
	out, err := r.ReadRequest()
	if err != nil {
		t.Fatalf("ReadRequest returned error: %v", err)
	}
	if out.Type != in.Type || out.ScriptPath != in.ScriptPath || len(out.ScriptArgs) != 1 {
		t.Fatalf("request round trip mismatch: %#v", out)
	}
}

func TestEventRoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)
	r := NewReader(buf)

	in := Event{Type: EventResult, RunID: "run-1", AtomicExitCode: 21, ScriptExitCode: 1, Message: "conflict"}
	if err := w.WriteEvent(in); err != nil {
		t.Fatalf("WriteEvent returned error: %v", err)
	}
	out, err := r.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent returned error: %v", err)
	}
	if out.Type != in.Type || out.AtomicExitCode != in.AtomicExitCode || out.Message != in.Message {
		t.Fatalf("event round trip mismatch: %#v", out)
	}
}

func TestStreamEventWriter(t *testing.T) {
	var events []Event
	writer := &StreamEventWriter{
		Kind:  EventStdout,
		RunID: "run-2",
		Sink: func(ev Event) error {
			events = append(events, ev)
			return nil
		},
	}
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	data, err := DecodeData(events[0].DataB64)
	if err != nil {
		t.Fatalf("DecodeData returned error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected decoded data %q", data)
	}
}
