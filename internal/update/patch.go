package update

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/database"
	"github.com/justjack1521/mevpatch/internal/patch"
	uuid "github.com/satori/go.uuid"
	"os"
	"sync"
)

type RemoteFileMergeJob struct {
	FileID            uuid.UUID
	NormalPath        string
	PatchFileTempPath string
}

type RemoteFileMergeWorker struct {
	group       *sync.WaitGroup
	application string
	repository  *database.PatchingRepository
	merger      *patch.Merger
	patches     <-chan *RemoteFileMergeJob
	commits     chan<- *FileMetadataCommitJob
	errors      chan<- error
}

func NewRemoteFileMergeWorker(group *sync.WaitGroup, app string, merger *patch.Merger, input <-chan *RemoteFileMergeJob, output chan<- *FileMetadataCommitJob, errors chan<- error) *RemoteFileMergeWorker {
	return &RemoteFileMergeWorker{group: group, application: app, merger: merger, patches: input, commits: output, errors: errors}
}

func (w *RemoteFileMergeWorker) Run() {
	defer w.group.Done()
	for job := range w.patches {
		fmt.Println(fmt.Sprintf("[Remote Patch] Started: %s", job.NormalPath))
		if err := w.run(job); err != nil {
			fmt.Println(fmt.Sprintf("[Remote Patch] Failed: %s: %s", job.NormalPath, err))
			w.errors <- err
			continue
		}
		fmt.Println(fmt.Sprintf("[Remote Patch] Completed: %s", job.NormalPath))
	}
}

func (w *RemoteFileMergeWorker) run(job *RemoteFileMergeJob) error {

	destination, err := PersistentPath(job.NormalPath)

	if _, err := os.Stat(job.PatchFileTempPath); os.IsNotExist(err) {
		return fmt.Errorf("patch file does not exist: %s", job.PatchFileTempPath)
	}

	if _, err := os.Stat(destination); os.IsNotExist(err) {
		return fmt.Errorf("target file does not exist: %s", destination)
	}

	if err != nil {
		return err
	}
	if err := w.merger.Merge(job.PatchFileTempPath, destination); err != nil {
		return err
	}
	return nil
}

type RemoteFileMergeWorkerGroup struct {
	app     string
	group   *sync.WaitGroup
	channel chan *RemoteFileMergeJob
	count   int
}

func NewRemoteFileMergeWorkerGroup(app string, count int) *RemoteFileMergeWorkerGroup {
	return &RemoteFileMergeWorkerGroup{
		app:     app,
		group:   &sync.WaitGroup{},
		channel: make(chan *RemoteFileMergeJob, count),
		count:   count,
	}
}

func (g *RemoteFileMergeWorkerGroup) Start(merger *patch.Merger, output chan<- *FileMetadataCommitJob, errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewRemoteFileMergeWorker(g.group, g.app, merger, g.channel, output, errors)
		go worker.Run()
	}
}

func (g *RemoteFileMergeWorkerGroup) Wait() {
	g.group.Wait()
}
