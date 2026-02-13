package diff

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

func ScanUpperDir(upperDir, mountPoint string) ([]journal.Operation, error) {
	opsByPath := map[string]journal.Operation{}
	opaqueDirs := map[string]bool{}

	err := filepath.WalkDir(upperDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(upperDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := filepath.Base(rel)
		dirRel := filepath.Dir(rel)
		if base == ".wh..wh..opq" {
			opaquePath := relToAbsPath(mountPoint, dirRel)
			op := opsByPath[opaquePath]
			op.Kind = journal.OperationUpsert
			op.Path = opaquePath
			op.SourcePath = filepath.Join(upperDir, dirRel)
			op.NodeType = journal.NodeDirectory
			op.Opaque = true
			opsByPath[opaquePath] = op
			opaqueDirs[opaquePath] = true
			return nil
		}
		if strings.HasPrefix(base, ".wh.") {
			targetName := strings.TrimPrefix(base, ".wh.")
			deletePath := relToAbsPath(mountPoint, filepath.Join(dirRel, targetName))
			opsByPath[deletePath] = journal.Operation{Kind: journal.OperationDelete, Path: deletePath, NodeType: journal.NodeUnknown}
			return nil
		}
		if isWhiteoutDevice(path, d) {
			deletePath := relToAbsPath(mountPoint, rel)
			opsByPath[deletePath] = journal.Operation{Kind: journal.OperationDelete, Path: deletePath, NodeType: journal.NodeUnknown}
			return nil
		}
		absPath := relToAbsPath(mountPoint, rel)
		nodeType, err := detectNodeType(path, d)
		if err != nil {
			return err
		}
		if nodeType == journal.NodeDirectory {
			existing, hasExisting := opsByPath[absPath]
			// Directory copy-up is common for nested file edits; avoid committing massive parent trees
			// unless the directory is explicitly opaque or was created empty.
			if !hasExisting || !existing.Opaque {
				isEmpty, err := isEmptyDir(path)
				if err != nil {
					return err
				}
				if !isEmpty {
					return nil
				}
			}
		}
		op := journal.Operation{Kind: journal.OperationUpsert, Path: absPath, SourcePath: path, NodeType: nodeType}
		if opaqueDirs[absPath] {
			op.Opaque = true
		}
		if existing, ok := opsByPath[absPath]; ok && existing.Kind == journal.OperationDelete {
			return nil
		}
		if existing, ok := opsByPath[absPath]; ok {
			op.Opaque = op.Opaque || existing.Opaque
		}
		opsByPath[absPath] = op
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk upperdir %s: %w", upperDir, err)
	}

	ops := make([]journal.Operation, 0, len(opsByPath))
	for _, op := range opsByPath {
		ops = append(ops, op)
	}
	return ops, nil
}

func Plan(ops []journal.Operation) []journal.Operation {
	ordered := make([]journal.Operation, len(ops))
	copy(ordered, ops)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]

		if left.Kind != right.Kind {
			if left.Kind == journal.OperationUpsert {
				return true
			}
			return false
		}

		if left.Kind == journal.OperationUpsert {
			leftIsDir := left.NodeType == journal.NodeDirectory
			rightIsDir := right.NodeType == journal.NodeDirectory
			if leftIsDir != rightIsDir {
				return leftIsDir
			}
			if pathDepth(left.Path) != pathDepth(right.Path) {
				return pathDepth(left.Path) < pathDepth(right.Path)
			}
			return left.Path < right.Path
		}

		if pathDepth(left.Path) != pathDepth(right.Path) {
			return pathDepth(left.Path) > pathDepth(right.Path)
		}
		return left.Path < right.Path
	})
	return ordered
}

func relToAbsPath(prefix, rel string) string {
	rel = filepath.Clean(rel)
	if rel == "." {
		rel = ""
	}
	prefix = filepath.Clean("/" + strings.TrimPrefix(prefix, "/"))
	if prefix == "/" {
		if rel == "" {
			return "/"
		}
		return filepath.Clean("/" + rel)
	}
	if rel == "" {
		return prefix
	}
	return filepath.Clean(filepath.Join(prefix, rel))
}

func detectNodeType(path string, d fs.DirEntry) (journal.NodeType, error) {
	if d.IsDir() {
		return journal.NodeDirectory, nil
	}
	if d.Type()&os.ModeSymlink != 0 {
		return journal.NodeSymlink, nil
	}
	if d.Type().IsRegular() {
		return journal.NodeFile, nil
	}
	info, err := d.Info()
	if err != nil {
		return journal.NodeUnknown, err
	}
	if info.Mode().IsRegular() {
		return journal.NodeFile, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return journal.NodeSymlink, nil
	}
	if info.IsDir() {
		return journal.NodeDirectory, nil
	}
	return journal.NodeUnknown, nil
}

func isWhiteoutDevice(path string, d fs.DirEntry) bool {
	if d.Type()&os.ModeCharDevice == 0 {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return st.Rdev == 0
}

func pathDepth(path string) int {
	if path == "/" {
		return 0
	}
	return len(strings.Split(strings.Trim(path, "/"), "/"))
}

func isEmptyDir(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".wh.") {
			continue
		}
		return false, nil
	}
	return true, nil
}
