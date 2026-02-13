package journal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type OperationKind string

const (
	OperationUpsert OperationKind = "upsert"
	OperationDelete OperationKind = "delete"
)

type NodeType string

const (
	NodeUnknown   NodeType = "unknown"
	NodeFile      NodeType = "file"
	NodeDirectory NodeType = "directory"
	NodeSymlink   NodeType = "symlink"
)

type Baseline struct {
	Exists  bool        `json:"exists"`
	Mode    os.FileMode `json:"mode"`
	UID     uint32      `json:"uid"`
	GID     uint32      `json:"gid"`
	Size    int64       `json:"size"`
	CTimeNs int64       `json:"ctime_ns"`
	MTimeNs int64       `json:"mtime_ns"`
	Inode   uint64      `json:"inode"`
	Dev     uint64      `json:"dev"`
}

type Operation struct {
	Kind       OperationKind `json:"kind"`
	Path       string        `json:"path"`
	SourcePath string        `json:"source_path,omitempty"`
	Baseline   Baseline      `json:"baseline"`
	NodeType   NodeType      `json:"node_type"`
	Opaque     bool          `json:"opaque,omitempty"`
}

type BackupRef struct {
	Exists bool   `json:"exists"`
	Path   string `json:"path"`
}

type Journal struct {
	RunID         string               `json:"run_id"`
	State         string               `json:"state"`
	Ops           []Operation          `json:"ops"`
	AppliedIndex  int                  `json:"applied_index"`
	BackupRefs    map[string]BackupRef `json:"backup_refs"`
	StartedAt     time.Time            `json:"started_at"`
	TxnStart      time.Time            `json:"txn_start"`
	RunDir        string               `json:"run_dir"`
	UpperDirs     []string             `json:"upper_dirs"`
	BackupDir     string               `json:"backup_dir"`
	KeepArtifacts bool                 `json:"keep_artifacts"`
}

const (
	StateCommitting  = "committing"
	StateCommitted   = "committed"
	StateRollingBack = "rolling_back"
	StateRolledBack  = "rolled_back"
)

func Save(path string, j *Journal) error {
	if j == nil {
		return errors.New("journal is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}
	blob, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(blob, '\n'), 0o600); err != nil {
		return fmt.Errorf("write temporary journal: %w", err)
	}
	if err := fsyncFile(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename journal: %w", err)
	}
	return fsyncDir(filepath.Dir(path))
}

func Load(path string) (*Journal, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	var j Journal
	if err := json.Unmarshal(blob, &j); err != nil {
		return nil, fmt.Errorf("unmarshal journal: %w", err)
	}
	if j.BackupRefs == nil {
		j.BackupRefs = map[string]BackupRef{}
	}
	return &j, nil
}

func ListPending(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read journal directory: %w", err)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		j, err := Load(path)
		if err != nil {
			return nil, err
		}
		if j.State == StateCommitted || j.State == StateRolledBack {
			continue
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func fsyncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for fsync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync file: %w", err)
	}
	return nil
}

func fsyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dir for fsync: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync dir: %w", err)
	}
	return nil
}
