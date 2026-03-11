package update

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	uuid "github.com/satori/go.uuid"
	"sync"
)

type FileCacheValidateJob struct {
	JobID uuid.UUID
	file  *mevmanifest.File
	cache file.LocalFile
	mode  UpdateMode
}

func NewFileCacheValidateJob(file *mevmanifest.File, local file.LocalFile, mode UpdateMode) *FileCacheValidateJob {
	return &FileCacheValidateJob{
		JobID: uuid.FromStringOrNil(file.Id),
		file:  file,
		cache: local,
		mode:  mode,
	}
}

type FileCacheValidateWorker struct {
	group       *sync.WaitGroup
	application string
	validates   <-chan *FileCacheValidateJob
	output      chan<- *FileCategorisationResult
	errors      chan<- error
	done        chan<- bool
}

func NewFileCacheValidateWorker(group *sync.WaitGroup, app string, input <-chan *FileCacheValidateJob, output chan<- *FileCategorisationResult, errors chan<- error, done chan<- bool) *FileCacheValidateWorker {
	return &FileCacheValidateWorker{group: group, application: app, validates: input, output: output, errors: errors, done: done}
}

func (w *FileCacheValidateWorker) Run() {
	defer w.group.Done()
	for job := range w.validates {
		fmt.Println(fmt.Sprintf("[Remote Validation] Started: %s", job.file.Path))
		if err := w.run(job); err != nil {
			fmt.Println(fmt.Sprintf("[Remote Failed] Failed: %s", job.file.Path))
			w.errors <- err
			continue
		}
		fmt.Println(fmt.Sprintf("[Remote Validation] Completed: %s", job.file.Path))

	}
}

func (w *FileCacheValidateWorker) run(job *FileCacheValidateJob) error {

	defer func() {
		w.done <- true
	}()

	if job.mode == UpdateModeForceFull {
		w.output <- &FileCategorisationResult{
			Category: FileResultDownload,
			Reason:   FileCategorisationReasonForceFull,
			File:     job.file,
		}
		return nil
	}

	if job.cache.Zero() {
		w.output <- &FileCategorisationResult{
			Category: FileResultDownload,
			Reason:   FileCategorisationReasonNotFoundDatabase,
			File:     job.file,
		}
		return nil
	}

	if job.cache.Size != job.file.Size || job.cache.Checksum != job.file.Checksum {
		if job.mode == UpdateModeRebase {
			w.output <- &FileCategorisationResult{
				Category: FileResultDownload,
				Reason:   FileCategorisationReasonDatabaseMismatch,
				File:     job.file,
			}
		} else {
			w.output <- &FileCategorisationResult{
				Category: FileResultPatch,
				Reason:   FileCategorisationReasonDatabaseMismatch,
				File:     job.file,
			}
		}
	}

	return nil

}

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

func (g *FileCacheValidateWorkerGroup) Start(output chan<- *FileCategorisationResult, errors chan<- error, done chan<- bool) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewFileCacheValidateWorker(g.group, g.app, g.channel, output, errors, done)
		go worker.Run()
	}
}

func (g *FileCacheValidateWorkerGroup) Wait() {
	g.group.Wait()
}
