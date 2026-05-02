package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
)

// RebuildStateStep regenerates the Files list in the local InstallState if
// UpdatedAt is zero — indicating the state was written by an older version of
// the patcher that didn't track file metadata properly.
//
// It walks the install directory, checksums every file it finds, and saves the
// rebuilt state before any planning or patching begins.
type RebuildStateStep struct{}

func NewRebuildStateStep() *RebuildStateStep { return &RebuildStateStep{} }

func (s *RebuildStateStep) Run(ctx *Context, o *Orchestrator) error {
	if !ctx.State.UpdatedAt.IsZero() {
		return nil
	}

	if ctx.CurrentVersion.Zero() {
		return nil
	}

	o.SendPrimaryStatusUpdate("Rebuilding local file index...")
	o.ResetPrimaryProgress()
	o.ResetSecondaryProgress()

	root, err := file.PersistentPath(ctx.ApplicationName, "")
	if err != nil {
		return fmt.Errorf("resolving install root: %w", err)
	}
	root = filepath.Clean(root)

	// Collect all files first so we can show progress.
	var paths []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".version" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning install directory: %w", err)
	}

	o.SendSecondaryStatusUpdate(fmt.Sprintf("Indexing %d files...", len(paths)))

	var files []file.LocalFile
	for i, path := range paths {
		checksum, err := patch.GetChecksumForPath(path)
		if err != nil {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}

		files = append(files, file.LocalFile{
			Path:      filepath.ToSlash(rel),
			Size:      info.Size(),
			Checksum:  checksum,
			Timestamp: info.ModTime().UTC(),
		})

		o.SetPrimaryProgress(float32(i+1) / float32(len(paths)))
	}

	ctx.State.Files = files
	ctx.State.UpdatedAt = time.Now().UTC()

	if err := patch.SaveInstallState(ctx.ApplicationName, ctx.State); err != nil {
		return fmt.Errorf("saving rebuilt state: %w", err)
	}

	o.SendPrimaryStatusUpdate("File index rebuilt")
	o.SendSecondaryStatusUpdate(fmt.Sprintf("%d files indexed", len(files)))

	return nil
}
