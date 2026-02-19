package ipc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const (
	Version = 1

	RequestRun     = "run"
	RequestRecover = "recover"

	EventStart  = "start"
	EventStdout = "stdout"
	EventStderr = "stderr"
	EventResult = "result"
	EventError  = "error"
)

type Request struct {
	Type          string            `json:"type"`
	Version       int               `json:"version"`
	ScriptPath    string            `json:"script_path,omitempty"`
	ScriptArgs    []string          `json:"script_args,omitempty"`
	CWD           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	KeepArtifacts bool              `json:"keep_artifacts,omitempty"`
	Verbose       bool              `json:"verbose,omitempty"`
}

type Event struct {
	Type           string `json:"type"`
	RunID          string `json:"run_id,omitempty"`
	AtomicExitCode int    `json:"atomic_exit_code,omitempty"`
	ScriptExitCode int    `json:"script_exit_code,omitempty"`
	Message        string `json:"message,omitempty"`
	DataB64        string `json:"data_b64,omitempty"`
}

type Writer struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{enc: json.NewEncoder(w)}
}

func (w *Writer) WriteRequest(req Request) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(req)
}

func (w *Writer) WriteEvent(ev Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(ev)
}

type Reader struct {
	dec *json.Decoder
}

func NewReader(r io.Reader) *Reader {
	return &Reader{dec: json.NewDecoder(r)}
}

func (r *Reader) ReadRequest() (Request, error) {
	var req Request
	if err := r.dec.Decode(&req); err != nil {
		return Request{}, err
	}
	return req, nil
}

func (r *Reader) ReadEvent() (Event, error) {
	var ev Event
	if err := r.dec.Decode(&ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}

type StreamEventWriter struct {
	Kind  string
	RunID string
	Sink  func(Event) error
}

func (w *StreamEventWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.Sink == nil {
		return 0, fmt.Errorf("stream sink is nil")
	}
	ev := Event{Type: w.Kind, RunID: w.RunID, DataB64: base64.StdEncoding.EncodeToString(p)}
	if err := w.Sink(ev); err != nil {
		return 0, err
	}
	return len(p), nil
}

func DecodeData(dataB64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(dataB64)
}
