package patch

import (
	"context"
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"sync"
	"time"
)

type FileMetadataCommitJob struct {
	Path     string
	Size     int64
	Checksum string
}

func NewFileMetadataCommitJob(path string, size int64, checksum string) *FileMetadataCommitJob {
	return &FileMetadataCommitJob{Path: path, Size: size, Checksum: checksum}
}

type FileMetadataCommitWorker struct {
	app        string
	repository file.Repository
	group      *sync.WaitGroup
	commits    <-chan *FileMetadataCommitJob
	errors     chan<- error
}

func NewFileMetadataCommitWorker(app string, repo file.Repository, group *sync.WaitGroup, input <-chan *FileMetadataCommitJob, errors chan<- error) *FileMetadataCommitWorker {
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
	if err := w.repository.CreateApplicationFile(context.Background(), w.app, job.Path, job.Size, job.Checksum, time.Now().UTC()); err != nil {
		fmt.Println(fmt.Sprintf("[File Commit] Failed: %s", job.Path))
		return err
	}
	return nil
}

type FileMetadataCommitWorkerGroup struct {
	repo    file.Repository
	group   *sync.WaitGroup
	Channel chan *FileMetadataCommitJob
	count   int
}

func NewFileMetadataCommitWorkerGroup(repo file.Repository, count int) *FileMetadataCommitWorkerGroup {
	return &FileMetadataCommitWorkerGroup{
		repo:    repo,
		group:   &sync.WaitGroup{},
		Channel: make(chan *FileMetadataCommitJob, count),
		count:   count,
	}
}

func (g *FileMetadataCommitWorkerGroup) Start(app string, errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewFileMetadataCommitWorker(app, g.repo, g.group, g.Channel, errors)
		go worker.Run()
	}
}

func (g *FileMetadataCommitWorkerGroup) Wait() {
	g.group.Wait()
}
