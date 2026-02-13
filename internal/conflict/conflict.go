package conflict

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

type FileState struct {
	Exists bool
	CTime  time.Time
}

type StatFunc func(path string) (FileState, error)

func Check(ops []journal.Operation, txnStart time.Time, statFn StatFunc) error {
	if statFn == nil {
		statFn = StatPath
	}
	paths := conflictPaths(ops)
	for _, path := range paths {
		st, err := statFn(path)
		if err != nil {
			return fmt.Errorf("stat conflict path %s: %w", path, err)
		}
		if st.Exists && st.CTime.After(txnStart) {
			return fmt.Errorf("conflict detected on %s", path)
		}
	}
	return nil
}

func AttachBaselines(ops []journal.Operation) ([]journal.Operation, error) {
	out := make([]journal.Operation, len(ops))
	copy(out, ops)
	for i := range out {
		baseline, err := BaselineForPath(out[i].Path)
		if err != nil {
			return nil, err
		}
		out[i].Baseline = baseline
	}
	return out, nil
}

func BaselineForPath(path string) (journal.Baseline, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return journal.Baseline{Exists: false}, nil
		}
		return journal.Baseline{}, err
	}
	uid, gid, inode, dev, ctimeNs, mtimeNs := statFields(info)
	return journal.Baseline{
		Exists:  true,
		Mode:    info.Mode(),
		UID:     uid,
		GID:     gid,
		Size:    info.Size(),
		CTimeNs: ctimeNs,
		MTimeNs: mtimeNs,
		Inode:   inode,
		Dev:     dev,
	}, nil
}

func StatPath(path string) (FileState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileState{Exists: false}, nil
		}
		return FileState{}, err
	}
	_, _, _, _, ctimeNs, _ := statFields(info)
	ctime := info.ModTime()
	if ctimeNs > 0 {
		ctime = time.Unix(0, ctimeNs)
	}
	return FileState{Exists: true, CTime: ctime}, nil
}

func conflictPaths(ops []journal.Operation) []string {
	seen := map[string]bool{}
	for _, op := range ops {
		if op.Path == "" {
			continue
		}
		clean := cleanAbs(op.Path)
		for _, path := range parentChain(clean) {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func parentChain(path string) []string {
	chain := []string{path}
	for path != "/" {
		path = filepath.Dir(path)
		if path == "." || path == "" {
			path = "/"
		}
		chain = append(chain, path)
	}
	return chain
}

func cleanAbs(path string) string {
	path = "/" + strings.TrimPrefix(path, "/")
	return filepath.Clean(path)
}
