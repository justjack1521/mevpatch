package patch

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	"os"
	"sync"
)

// RemoteFileMergeJob describes a single patch-apply operation.
type RemoteFileMergeJob struct {
	// ParentFile is the manifest entry for the file being patched.
	ParentFile *mevmanifest.File
	// PatchFileTempPath is where the extracted .jdf patch file lives on disk.
	PatchFileTempPath string
}

// RemoteFileMergeWorker applies patch jobs from a channel, verifies the
// result checksum, then forwards a commit job on success.
type RemoteFileMergeWorker struct {
	group       *sync.WaitGroup
	application string
	tool        *MergeTool
	patches     <-chan *RemoteFileMergeJob
	commits     chan<- *FileMetadataCommitJob
	errors      chan<- error
}

func NewRemoteFileMergeWorker(
	group *sync.WaitGroup,
	app string,
	tool *MergeTool,
	input <-chan *RemoteFileMergeJob,
	output chan<- *FileMetadataCommitJob,
	errors chan<- error,
) *RemoteFileMergeWorker {
	return &RemoteFileMergeWorker{
		group: group, application: app, tool: tool,
		patches: input, commits: output, errors: errors,
	}
}

func (w *RemoteFileMergeWorker) Run() {
	defer w.group.Done()
	for job := range w.patches {
		fmt.Printf("[Merge] Applying patch: %s\n", job.ParentFile.Path)
		if err := w.run(job); err != nil {
			fmt.Printf("[Merge] Failed: %s: %v\n", job.ParentFile.Path, err)
			w.errors <- err
		} else {
			fmt.Printf("[Merge] Done: %s\n", job.ParentFile.Path)
		}
	}
}

func (w *RemoteFileMergeWorker) run(job *RemoteFileMergeJob) error {
	defer os.Remove(job.PatchFileTempPath)

	destination, err := file.PersistentPath(w.application, job.ParentFile.Path)
	if err != nil {
		return fmt.Errorf("resolving path for %s: %w", job.ParentFile.Path, err)
	}

	// Skip if already at the target checksum (e.g. file was already downloaded).
	current, err := GetChecksumForPath(destination)
	if err != nil {
		return fmt.Errorf("checksumming %s before patch: %w", destination, err)
	}
	if current == job.ParentFile.Checksum {
		fmt.Printf("[Merge] Already up to date, skipping: %s\n", job.ParentFile.Path)
		return nil
	}

	if err := file.CanReadAtPath(job.PatchFileTempPath); err != nil {
		return fmt.Errorf("patch file not readable %s: %w", job.PatchFileTempPath, err)
	}
	if err := file.CanReadAtPath(destination); err != nil {
		return fmt.Errorf("target file not readable %s: %w", destination, err)
	}

	if err := w.tool.Apply(destination, job.PatchFileTempPath); err != nil {
		return err
	}

	result, err := GetChecksumForPath(destination)
	if err != nil {
		return fmt.Errorf("checksumming %s after patch: %w", destination, err)
	}
	if result != job.ParentFile.Checksum {
		return fmt.Errorf("post-patch checksum mismatch for %s: expected %s got %s",
			job.ParentFile.Path, job.ParentFile.Checksum, result)
	}

	w.commits <- &FileMetadataCommitJob{
		Path:     job.ParentFile.Path,
		Size:     job.ParentFile.Size,
		Checksum: result,
	}
	return nil
}

// RemoteFileMergeWorkerGroup manages a pool of merge workers.
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
		w := NewRemoteFileMergeWorker(g.group, app, g.tool, g.channel, output, errors)
		go w.Run()
	}
}

func (g *RemoteFileMergeWorkerGroup) Wait() {
	g.group.Wait()
}
