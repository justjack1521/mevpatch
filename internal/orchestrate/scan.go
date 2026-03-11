package orchestrate

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	StatusUpdateScanningFiles = "Scanning files in directory"
)

type DirectoryScanStep struct {
	commiters *patch.FileMetadataCommitWorkerGroup
	errors    chan error
}

func NewDirectoryScanStep() *DirectoryScanStep {
	return &DirectoryScanStep{
		errors: make(chan error, 10),
	}
}

func (s *DirectoryScanStep) Run(ctx *Context, orchestrator *Orchestrator) error {

	orchestrator.SendPrimaryStatusUpdate(StatusUpdateScanningFiles)

	s.commiters = patch.NewFileMetadataCommitWorkerGroup(ctx.LocalState, 10)

	defer close(s.errors)

	go func() {
		for err := range s.errors {
			fmt.Printf("Directory Scanning Error: %v\n", err)
		}
	}()

	s.commiters.Start(ctx.ApplicationName, s.errors)

	start, err := file.PersistentPath(ctx.ApplicationName, "")
	if err != nil {
		return err
	}

	if err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {

		var normal = filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, start), "\\"))

		fmt.Println(fmt.Sprintf("[File Scan] Started: %s", normal))

		if err != nil {
			fmt.Printf("Error accessing path %s: %v\n", path, err)
			return nil
		}

		if d.IsDir() && filepath.Base(path) == "patching" {
			return filepath.SkipDir
		}

		if d.IsDir() && ctx.ApplicationName == "launcher" && filepath.Base(path) == "game" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		checksum, err := patch.GetChecksumForPath(path)
		if err != nil {
			return err
		}

		fmt.Println(fmt.Sprintf("[File Scan] Completed: %s", normal))

		s.commiters.Channel <- patch.NewFileMetadataCommitJob(normal, info.Size(), checksum)

		return nil

	}); err != nil {
		fmt.Printf("Directory Scanning Error: %v\n", err)
		s.errors <- err
	}

	close(s.commiters.Channel)
	s.commiters.Wait()

	return nil

}
