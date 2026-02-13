package commit

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/ShriKaranHanda/atomic/internal/journal"
)

type Engine struct {
	RootPrefix string
}

func (e Engine) Apply(journalPath string, j *journal.Journal) error {
	if j == nil {
		return errors.New("journal is nil")
	}
	if j.BackupRefs == nil {
		j.BackupRefs = map[string]journal.BackupRef{}
	}
	j.State = journal.StateCommitting
	if err := journal.Save(journalPath, j); err != nil {
		return err
	}

	for idx := j.AppliedIndex + 1; idx < len(j.Ops); idx++ {
		op := j.Ops[idx]
		if err := e.backupPath(j, op.Path); err != nil {
			e.rollbackIgnoringErrors(journalPath, j)
			return fmt.Errorf("backup %s: %w", op.Path, err)
		}
		if err := e.applyOperation(op); err != nil {
			e.rollbackIgnoringErrors(journalPath, j)
			return fmt.Errorf("apply %s: %w", op.Path, err)
		}
		j.AppliedIndex = idx
		if err := journal.Save(journalPath, j); err != nil {
			e.rollbackIgnoringErrors(journalPath, j)
			return err
		}
	}

	j.State = journal.StateCommitted
	if err := journal.Save(journalPath, j); err != nil {
		return err
	}
	return nil
}

func (e Engine) Rollback(journalPath string, j *journal.Journal) error {
	j.State = journal.StateRollingBack
	if err := journal.Save(journalPath, j); err != nil {
		return err
	}
	paths := make([]string, 0, len(j.BackupRefs))
	for path := range j.BackupRefs {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		return pathDepth(paths[i]) > pathDepth(paths[j])
	})

	for _, path := range paths {
		ref := j.BackupRefs[path]
		target := e.targetPath(path)
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove target during rollback %s: %w", target, err)
		}
		if ref.Exists {
			if err := copyPath(ref.Path, target); err != nil {
				return fmt.Errorf("restore target %s: %w", target, err)
			}
		}
	}
	j.State = journal.StateRolledBack
	return journal.Save(journalPath, j)
}

func (e Engine) rollbackIgnoringErrors(journalPath string, j *journal.Journal) {
	_ = e.Rollback(journalPath, j)
}

func (e Engine) backupPath(j *journal.Journal, opPath string) error {
	if _, ok := j.BackupRefs[opPath]; ok {
		return nil
	}
	if err := ensureDir(j.BackupDir); err != nil {
		return err
	}
	target := e.targetPath(opPath)
	backup := filepath.Join(j.BackupDir, strings.TrimPrefix(filepath.Clean(opPath), "/"))
	if backup == j.BackupDir {
		backup = filepath.Join(j.BackupDir, "root")
	}
	if _, err := os.Lstat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			j.BackupRefs[opPath] = journal.BackupRef{Exists: false}
			return nil
		}
		return err
	}
	if err := copyPathWithSkips(target, backup, []string{j.BackupDir}); err != nil {
		return err
	}
	j.BackupRefs[opPath] = journal.BackupRef{Exists: true, Path: backup}
	return nil
}

func (e Engine) applyOperation(op journal.Operation) error {
	target := e.targetPath(op.Path)
	switch op.Kind {
	case journal.OperationDelete:
		if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncParent(target)
	case journal.OperationUpsert:
		return e.applyUpsert(op, target)
	default:
		return fmt.Errorf("unsupported operation kind %q", op.Kind)
	}
}

func (e Engine) applyUpsert(op journal.Operation, target string) error {
	source := op.SourcePath
	if source == "" {
		return fmt.Errorf("empty source path for upsert %s", op.Path)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	switch op.NodeType {
	case journal.NodeDirectory:
		if op.Opaque {
			if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err := ensureDir(target); err != nil {
			return err
		}
		if err := os.Chmod(target, info.Mode().Perm()); err != nil {
			return err
		}
		if err := copyDirContents(source, target); err != nil {
			return err
		}
		if err := chownLikeSource(target, info); err != nil {
			return err
		}
		return syncParent(target)
	case journal.NodeFile:
		if err := ensureDir(filepath.Dir(target)); err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		tmpPath := target + ".atomic.tmp"
		if err := copyRegularFile(source, tmpPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, target); err != nil {
			return err
		}
		if err := chownLikeSource(target, info); err != nil {
			return err
		}
		if err := syncFile(target); err != nil {
			return err
		}
		return syncParent(target)
	case journal.NodeSymlink:
		if err := ensureDir(filepath.Dir(target)); err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if err := os.Symlink(link, target); err != nil {
			return err
		}
		return syncParent(target)
	default:
		return fmt.Errorf("unsupported node type %q", op.NodeType)
	}
}

func (e Engine) targetPath(path string) string {
	clean := filepath.Clean("/" + strings.TrimPrefix(path, "/"))
	if e.RootPrefix == "" {
		return clean
	}
	if clean == "/" {
		return e.RootPrefix
	}
	return filepath.Join(e.RootPrefix, strings.TrimPrefix(clean, "/"))
}

func copyDirContents(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		base := filepath.Base(rel)
		if strings.HasPrefix(base, ".wh.") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := ensureDir(target); err != nil {
				return err
			}
			if err := os.Chmod(target, info.Mode().Perm()); err != nil {
				return err
			}
			return chownLikeSource(target, info)
		}
		if d.Type().IsRegular() {
			if err := copyRegularFile(path, target); err != nil {
				return err
			}
			if err := chownLikeSource(target, info); err != nil {
				return err
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, target); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("unsupported node type in directory copy: %s", path)
	})
}

func copyRegularFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}

func copyPath(src, dst string) error {
	return copyPathWithSkips(src, dst, nil)
}

func copyPathWithSkips(src, dst string, skipPrefixes []string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	cleanSkips := make([]string, 0, len(skipPrefixes))
	for _, skip := range skipPrefixes {
		if skip == "" {
			continue
		}
		cleanSkips = append(cleanSkips, filepath.Clean(skip))
	}

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := ensureDir(dst); err != nil {
			return err
		}
		if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
			return err
		}
		if err := chownLikeSource(dst, info); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childSrc := filepath.Join(src, entry.Name())
			if shouldSkip(childSrc, cleanSkips) {
				continue
			}
			if err := copyPathWithSkips(childSrc, filepath.Join(dst, entry.Name()), cleanSkips); err != nil {
				return err
			}
		}
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if err := ensureDir(filepath.Dir(dst)); err != nil {
			return err
		}
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Symlink(link, dst)
	}
	return copyRegularFile(src, dst)
}

func shouldSkip(path string, skipPrefixes []string) bool {
	path = filepath.Clean(path)
	for _, prefix := range skipPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func chownLikeSource(target string, sourceInfo os.FileInfo) error {
	st, ok := sourceInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := os.Lchown(target, int(st.Uid), int(st.Gid)); err != nil {
		return err
	}
	return nil
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func syncParent(path string) error {
	return syncFile(filepath.Dir(path))
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func pathDepth(path string) int {
	if path == "/" {
		return 0
	}
	return len(strings.Split(strings.Trim(path, "/"), "/"))
}
