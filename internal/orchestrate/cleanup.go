package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/manifest"
)

// CleanupStep removes files from the install directory that are not listed
// in the manifest for the currently installed version.
//
// Always excluded (never deleted):
//   - The Patching/ directory and everything in it (patcher exe, .version files)
//
// Launcher-specific exclusions:
//   - The Game/ subdirectory and everything in it
type CleanupStep struct {
	Removed []string
}

func NewCleanupStep() *CleanupStep { return &CleanupStep{} }

func (s *CleanupStep) Run(ctx *Context, o *Orchestrator) error {
	if ctx.CurrentVersion.Zero() {
		return fmt.Errorf("no installed version found — run patch first")
	}

	o.SendPrimaryStatusUpdate("Cleaning up old files...")
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Fetching manifest for %s...", ctx.CurrentVersion.String()))

	mani, err := manifest.DownloadManifest(remoteHost, ctx.ApplicationName, ctx.CurrentVersion)
	if err != nil {
		return fmt.Errorf("downloading manifest for cleanup: %w", err)
	}

	// Build a set of all paths that should exist.
	keep := make(map[string]bool, len(mani.Files))
	for _, f := range mani.Files {
		absPath, err := file.PersistentPath(ctx.ApplicationName, f.Path)
		if err != nil {
			continue
		}
		keep[filepath.Clean(absPath)] = true
	}

	// Resolve the root directory for this app.
	root, err := file.PersistentPath(ctx.ApplicationName, "")
	if err != nil {
		return fmt.Errorf("resolving install root: %w", err)
	}
	root = filepath.Clean(root)

	// Resolve directories to always skip.
	patchingDir, err := file.PatcherDir()
	if err != nil {
		return fmt.Errorf("resolving patching dir: %w", err)
	}
	patchingDir = filepath.Clean(patchingDir)

	skipDirs := map[string]bool{
		patchingDir: true,
	}

	// For the launcher, also skip the Game subdirectory.
	if ctx.ApplicationName == "launcher" {
		gameDir, err := file.PersistentPath("game", "")
		if err == nil {
			skipDirs[filepath.Clean(gameDir)] = true
		}
	}

	o.SendSecondaryStatusUpdate("Scanning for orphaned files...")

	var toDelete []string

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		clean := filepath.Clean(path)

		// Skip the root itself.
		if clean == root {
			return nil
		}

		// Skip any directory in the exclusion list (and don't descend into it).
		if info.IsDir() {
			if skipDirs[clean] {
				return filepath.SkipDir
			}
			// Also skip subdirs of excluded dirs.
			for skipDir := range skipDirs {
				if strings.HasPrefix(clean, skipDir+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Always skip .version files (in case any live outside Patching/).
		if filepath.Ext(clean) == ".version" {
			return nil
		}

		if !keep[clean] {
			toDelete = append(toDelete, clean)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning install directory: %w", err)
	}

	if len(toDelete) == 0 {
		o.SendPrimaryStatusUpdate("Nothing to clean up")
		o.SendSecondaryStatusUpdate("All files accounted for")
		return nil
	}

	o.SendSecondaryStatusUpdate(fmt.Sprintf("Removing %d orphaned file(s)...", len(toDelete)))

	var failed int
	for _, path := range toDelete {
		rel, _ := filepath.Rel(root, path)
		if err := os.Remove(path); err != nil {
			o.SendLog("FAILED: %s (%v)", rel, err)
			failed++
		} else {
			o.SendLog("REMOVED: %s", rel)
			s.Removed = append(s.Removed, path)
		}
	}

	if failed > 0 {
		o.SendPrimaryStatusUpdate("Cleanup complete (with errors)")
		o.SendSecondaryStatusUpdate(fmt.Sprintf(
			"%d removed, %d failed", len(s.Removed), failed,
		))
	} else {
		o.SendPrimaryStatusUpdate("Cleanup complete")
		o.SendSecondaryStatusUpdate(fmt.Sprintf("%d orphaned file(s) removed", len(s.Removed)))
	}

	return nil
}
