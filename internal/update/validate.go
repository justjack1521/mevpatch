package update

import (
	"fmt"
	"sync"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
)

// UpdateMode controls how the planner handles files that differ from the manifest.
type UpdateMode int

const (
	// UpdateModeDefault: patch if a bundle exists for current version, otherwise download.
	UpdateModeDefault UpdateMode = iota
	// UpdateModeRebase: force full download for all differing files.
	// Used when current version is outside the patch window (no bundle available).
	UpdateModeRebase
)

// FileCacheValidateJob is one unit of work for the validate workers.
type FileCacheValidateJob struct {
	File      *mevmanifest.File
	CacheFile file.LocalFile // may be zero if not in state
	Mode      UpdateMode
	// HasBundle indicates whether a patch bundle exists for the current version.
	HasBundle bool
}

// FileCacheValidateWorker checks each file against local state and routes it
// to the appropriate result category.
type FileCacheValidateWorker struct {
	group       *sync.WaitGroup
	application string
	validates   <-chan *FileCacheValidateJob
	output      chan<- *FileCategorisationResult
	errors      chan<- error
}

func NewFileCacheValidateWorker(
	group *sync.WaitGroup,
	app string,
	input <-chan *FileCacheValidateJob,
	output chan<- *FileCategorisationResult,
	errors chan<- error,
) *FileCacheValidateWorker {
	return &FileCacheValidateWorker{
		group: group, application: app,
		validates: input, output: output, errors: errors,
	}
}

func (w *FileCacheValidateWorker) Run() {
	defer w.group.Done()
	for job := range w.validates {
		if err := w.run(job); err != nil {
			fmt.Printf("[Validate] Error for %s: %v\n", job.File.Path, err)
			w.errors <- err
		}
	}
}

func (w *FileCacheValidateWorker) run(job *FileCacheValidateJob) error {
	// Case 1: file not in local state at all → must download from source.
	if job.CacheFile.Zero() {
		w.output <- &FileCategorisationResult{
			Category: FileResultDownload,
			Reason:   ReasonNotOnDisk,
			File:     job.File,
		}
		return nil
	}

	// Case 2: cached metadata matches manifest → nothing to do.
	if job.CacheFile.Size == job.File.Size && job.CacheFile.Checksum == job.File.Checksum {
		w.output <- &FileCategorisationResult{
			Category: FileResultIgnore,
			Reason:   ReasonUpToDate,
			File:     job.File,
		}
		return nil
	}

	// File differs. Decide how to update it:

	// Case 3: rebase mode or no bundle available → full download.
	if job.Mode == UpdateModeRebase || !job.HasBundle {
		reason := ReasonNoPatchBundle
		if job.Mode == UpdateModeRebase {
			reason = ReasonRebase
		}
		w.output <- &FileCategorisationResult{Category: FileResultDownload, Reason: reason, File: job.File}
		return nil
	}

	// Case 4: bundle available → patch.
	w.output <- &FileCategorisationResult{Category: FileResultPatch, Reason: ReasonChecksumDiff, File: job.File}
	return nil
}

// FileCacheValidateWorkerGroup manages a pool of validate workers.
type FileCacheValidateWorkerGroup struct {
	app     string
	group   *sync.WaitGroup
	channel chan *FileCacheValidateJob
	count   int
}

func NewFileCacheValidateWorkerGroup(app string, count int) *FileCacheValidateWorkerGroup {
	return &FileCacheValidateWorkerGroup{
		app:     app,
		group:   &sync.WaitGroup{},
		channel: make(chan *FileCacheValidateJob, count),
		count:   count,
	}
}

func (g *FileCacheValidateWorkerGroup) Start(
	output chan<- *FileCategorisationResult,
	errors chan<- error,
) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		w := NewFileCacheValidateWorker(g.group, g.app, g.channel, output, errors)
		go w.Run()
	}
}

func (g *FileCacheValidateWorkerGroup) Wait() { g.group.Wait() }
