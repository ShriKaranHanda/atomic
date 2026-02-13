package recover

import (
	"fmt"
	"os"

	"github.com/ShriKaranHanda/atomic/internal/commit"
	"github.com/ShriKaranHanda/atomic/internal/journal"
)

func Run(journalDir string, rootPrefix string) error {
	pending, err := journal.ListPending(journalDir)
	if err != nil {
		return err
	}
	eng := commit.Engine{RootPrefix: rootPrefix}
	for _, path := range pending {
		j, err := journal.Load(path)
		if err != nil {
			return err
		}
		if err := eng.Apply(path, j); err != nil {
			if rbErr := eng.Rollback(path, j); rbErr != nil {
				return fmt.Errorf("resume commit failed: %v; rollback also failed: %w", err, rbErr)
			}
			return fmt.Errorf("resume commit failed and was rolled back: %w", err)
		}
		if err := finalize(path, j); err != nil {
			return err
		}
	}
	return nil
}

func finalize(journalPath string, j *journal.Journal) error {
	if !j.KeepArtifacts {
		if j.RunDir != "" {
			if err := os.RemoveAll(j.RunDir); err != nil {
				return err
			}
		}
		if j.BackupDir != "" {
			if err := os.RemoveAll(j.BackupDir); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(journalPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
