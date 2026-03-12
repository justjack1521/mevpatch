package patch

import (
	"fmt"
)

// RemotePatchFileMerger orchestrates parallel patch-apply operations.
type RemotePatchFileMerger struct {
	application string
	patchers    *RemoteFileMergeWorkerGroup
	commiters   *FileMetadataCommitWorkerGroup
	errors      chan error
}

func NewRemotePatchFileMerger(app string, tool *MergeTool, state *InstallState) *RemotePatchFileMerger {
	return &RemotePatchFileMerger{
		application: app,
		patchers:    NewRemoteFileMergeWorkerGroup(tool, 4),
		commiters:   NewFileMetadataCommitWorkerGroup(state, 4),
		errors:      make(chan error, 20),
	}
}

// Start applies all jobs concurrently and blocks until complete.
func (u *RemotePatchFileMerger) Start(jobs []*RemoteFileMergeJob) {
	defer close(u.errors)

	go func() {
		for err := range u.errors {
			fmt.Printf("[Patch Merger] Error: %v\n", err)
		}
	}()

	u.patchers.Start(u.application, u.commiters.Channel, u.errors)
	u.commiters.Start(u.application, u.errors)

	for _, job := range jobs {
		u.patchers.channel <- job
	}

	close(u.patchers.channel)
	u.patchers.Wait()

	close(u.commiters.Channel)
	u.commiters.Wait()
}
