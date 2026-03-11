package patch

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
)

type RemotePatchFileMerger struct {
	Application string
	patchers    *RemoteFileMergeWorkerGroup
	commiters   *FileMetadataCommitWorkerGroup
	errors      chan error
}

func NewRemotePatchFileMerger(app string, tool *MergeTool, files file.Repository) *RemotePatchFileMerger {
	return &RemotePatchFileMerger{
		Application: app,
		patchers:    NewRemoteFileMergeWorkerGroup(tool, 10),
		commiters:   NewFileMetadataCommitWorkerGroup(files, 10),
		errors:      make(chan error, 10),
	}
}

func (u *RemotePatchFileMerger) Start(jobs []*RemoteFileMergeJob) {

	defer close(u.errors)

	go func() {
		for err := range u.errors {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	u.patchers.Start(u.Application, u.commiters.Channel, u.errors)
	u.commiters.Start(u.Application, u.errors)

	for _, job := range jobs {
		u.patchers.channel <- job
	}

	close(u.patchers.channel)
	u.patchers.Wait()

	close(u.commiters.Channel)
	u.commiters.Wait()

}
