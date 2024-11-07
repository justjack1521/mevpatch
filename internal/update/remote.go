package update

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/database"
	"os"
	"sync"
)

type RemoteFileValidateJob struct {
	file *mevmanifest.File
}

func NewRemoteFileValidateJob(file *mevmanifest.File) *RemoteFileValidateJob {
	return &RemoteFileValidateJob{
		file: file,
	}
}

type RemoteFileValidateWorker struct {
	group       *sync.WaitGroup
	application string
	repository  *database.PatchingRepository
	validates   <-chan *RemoteFileValidateJob
	output      chan<- *FileCategorisationResult
	errors      chan<- error
}

func NewRemoteFileValidateWorker(group *sync.WaitGroup, app string, repo *database.PatchingRepository, input <-chan *RemoteFileValidateJob, output chan<- *FileCategorisationResult, errors chan<- error) *RemoteFileValidateWorker {
	return &RemoteFileValidateWorker{group: group, application: app, repository: repo, validates: input, output: output, errors: errors}
}

func (w *RemoteFileValidateWorker) Run() {
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

func (w *RemoteFileValidateWorker) run(job *RemoteFileValidateJob) error {

	destination, err := PersistentPath(job.file.Path)
	if err != nil {
		w.errors <- err
		return err
	}

	local, err := w.repository.GetApplicationFile(context.Background(), w.application, job.file.Path)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.output <- &FileCategorisationResult{
				Category: FileResultDownload,
				Reason:   FileCategorisationReasonNotFoundDatabase,
				File:     job.file,
			}
			return nil
		}
		w.errors <- err
		return err
	}

	stat, err := os.Stat(destination)
	if err != nil {
		w.output <- &FileCategorisationResult{
			Category: FileResultDownload,
			Reason:   FileCategorisationReasonNotFoundDisk,
			File:     job.file,
		}
		return nil
	}

	if local.Size != stat.Size() || stat.ModTime().UTC().Equal(local.Timestamp) == false {
		w.output <- &FileCategorisationResult{
			Category: FileResultDownload,
			Reason:   FileCategorisationReasonDatabaseMismatch,
			File:     job.file,
		}
		return nil
	}

	w.output <- &FileCategorisationResult{
		Category: FileResultIgnore,
		File:     job.file,
	}

	return nil

}

type RemoteFileValidateWorkerGroup struct {
	app     string
	repo    *database.PatchingRepository
	group   *sync.WaitGroup
	channel chan *RemoteFileValidateJob
	count   int
}

func NewRemoteFileValidateWorkerGroup(app string, repo *database.PatchingRepository, count int) *RemoteFileValidateWorkerGroup {
	return &RemoteFileValidateWorkerGroup{
		app:     app,
		repo:    repo,
		group:   &sync.WaitGroup{},
		channel: make(chan *RemoteFileValidateJob, count),
		count:   count,
	}
}

func (g *RemoteFileValidateWorkerGroup) Start(output chan<- *FileCategorisationResult, errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewRemoteFileValidateWorker(g.group, g.app, g.repo, g.channel, output, errors)
		go worker.Run()
	}
}

func (g *RemoteFileValidateWorkerGroup) Wait() {
	g.group.Wait()
}
