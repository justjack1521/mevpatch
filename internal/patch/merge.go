package patch

import (
	"errors"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	"os"
	"sync"
)

type RemoteFileMergeJob struct {
	ParentFile        *mevmanifest.File
	PatchFile         *mevmanifest.PatchFile
	PatchFileTempPath string
}

type RemoteFileMergeWorker struct {
	group       *sync.WaitGroup
	application string
	merger      *MergeTool
	patches     <-chan *RemoteFileMergeJob
	commits     chan<- *FileMetadataCommitJob
	errors      chan<- error
}

func NewRemoteFileMergeWorker(group *sync.WaitGroup, app string, merger *MergeTool, input <-chan *RemoteFileMergeJob, output chan<- *FileMetadataCommitJob, errors chan<- error) *RemoteFileMergeWorker {
	return &RemoteFileMergeWorker{group: group, application: app, merger: merger, patches: input, commits: output, errors: errors}
}

func (w *RemoteFileMergeWorker) Run() {
	defer w.group.Done()
	for job := range w.patches {
		fmt.Println(fmt.Sprintf("[Remote Patch] Started: %s", job.ParentFile.Path))
		applied, err := w.run(job)
		if err != nil {
			fmt.Println(fmt.Sprintf("[Remote Patch] Failed: %s: %s", job.ParentFile.Path, err))
			w.errors <- err
			continue
		}
		if applied == false {
			fmt.Println(fmt.Sprintf("[Remote Patch] Skipped: %s", job.ParentFile.Path))
			continue
		}
		fmt.Println(fmt.Sprintf("[Remote Patch] Completed: %s", job.ParentFile.Path))
	}
}

func (w *RemoteFileMergeWorker) run(job *RemoteFileMergeJob) (bool, error) {

	defer os.Remove(job.PatchFileTempPath)

	destination, err := file.PersistentPath(w.application, job.ParentFile.Path)
	if err != nil {
		return false, err
	}

	initial, err := GetChecksumForPath(destination)
	if err != nil {
		return false, err
	}

	if initial == job.ParentFile.Checksum {
		return false, nil
	}

	if err := file.CanReadAtPath(job.PatchFileTempPath); err != nil {
		return false, err
	}

	if err := file.CanReadAtPath(destination); err != nil {
		return false, err
	}

	if err := w.merger.Merge(destination, job.PatchFileTempPath); err != nil {
		return false, err
	}

	result, err := GetChecksumForPath(destination)
	if err != nil {
		return false, err
	}

	if result != job.ParentFile.Checksum {
		return true, errors.New(fmt.Sprintf("final checksum for %s does not match, expected %s got %s", job.ParentFile.Path, job.ParentFile.Checksum, result))
	}

	w.commits <- &FileMetadataCommitJob{
		Path:     job.ParentFile.Path,
		Size:     job.ParentFile.Size,
		Checksum: result,
	}

	return true, nil
}

type RemoteFileMergeWorkerGroup struct {
	tool    *MergeTool
	group   *sync.WaitGroup
	channel chan *RemoteFileMergeJob
	count   int
}

func NewRemoteFileMergeWorkerGroup(tool *MergeTool, count int) *RemoteFileMergeWorkerGroup {
	return &RemoteFileMergeWorkerGroup{
		tool:    tool,
		group:   &sync.WaitGroup{},
		channel: make(chan *RemoteFileMergeJob, count),
		count:   count,
	}
}

func (g *RemoteFileMergeWorkerGroup) Start(app string, output chan<- *FileMetadataCommitJob, errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewRemoteFileMergeWorker(g.group, app, g.tool, g.channel, output, errors)
		go worker.Run()
	}
}

func (g *RemoteFileMergeWorkerGroup) Wait() {
	g.group.Wait()
}
