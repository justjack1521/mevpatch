package update

import (
	"context"
	"fmt"
	"github.com/justjack1521/mevpatch/internal/database"
	"sync"
	"time"
)

type FileMetadataCommitJob struct {
	Path string
	Size int64
}

type FileMetadataCommitWorker struct {
	app        string
	repository *database.PatchingRepository
	group      *sync.WaitGroup
	commits    <-chan *FileMetadataCommitJob
	errors     chan<- error
}

func NewFileMetadataCommitWorker(app string, repo *database.PatchingRepository, group *sync.WaitGroup, input <-chan *FileMetadataCommitJob, errors chan<- error) *FileMetadataCommitWorker {
	return &FileMetadataCommitWorker{app: app, repository: repo, group: group, commits: input, errors: errors}
}

func (w *FileMetadataCommitWorker) Run() {
	defer w.group.Done()
	for job := range w.commits {
		fmt.Println(fmt.Sprintf("[File Commit] Started: %s", job.Path))
		if err := w.run(job); err != nil {
			fmt.Println(fmt.Sprintf("[File Commit] Failed: %s", job.Path))
			w.errors <- err
			continue
		}
		fmt.Println(fmt.Sprintf("[File Commit] Completed: %s", job.Path))
	}
}

func (w *FileMetadataCommitWorker) run(job *FileMetadataCommitJob) error {
	if err := w.repository.CreateApplicationFile(context.Background(), w.app, job.Path, job.Size, "", time.Now().UTC()); err != nil {
		fmt.Println(fmt.Sprintf("[File Commit] Failed: %s", job.Path))
		return err
	}
	return nil
}

type FileMetadataCommitWorkerGroup struct {
	app     string
	repo    *database.PatchingRepository
	group   *sync.WaitGroup
	channel chan *FileMetadataCommitJob
	count   int
}

func NewFileMetadataCommitWorkerGroup(app string, repo *database.PatchingRepository, count int) *FileMetadataCommitWorkerGroup {
	return &FileMetadataCommitWorkerGroup{
		app:     app,
		repo:    repo,
		group:   &sync.WaitGroup{},
		channel: make(chan *FileMetadataCommitJob, count),
		count:   count,
	}
}

func (g *FileMetadataCommitWorkerGroup) Start(errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewFileMetadataCommitWorker(g.app, g.repo, g.group, g.channel, errors)
		go worker.Run()
	}
}

func (g *FileMetadataCommitWorkerGroup) Wait() {
	g.group.Wait()
}
