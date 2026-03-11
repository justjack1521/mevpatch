package patch

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	uuid "github.com/satori/go.uuid"
)

type RemoteSourceFileDownloader struct {
	Application string
	sourcers    *RemoteFileSourceWorkerGroup
	mergers     *RemoteFileMergeWorkerGroup
	commiters   *FileMetadataCommitWorkerGroup
	errors      chan error
}

func NewRemoteSourceFileDownloader(app string, files file.Repository) *RemoteSourceFileDownloader {
	return &RemoteSourceFileDownloader{
		Application: app,
		sourcers:    NewRemoteFileSourceWorkerGroup(10),
		commiters:   NewFileMetadataCommitWorkerGroup(files, 10),
		errors:      make(chan error, 10),
	}
}

func (u *RemoteSourceFileDownloader) Start(files []*mevmanifest.File, progress chan<- float32) {

	defer close(u.errors)
	defer close(progress)

	go func() {
		for err := range u.errors {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	u.sourcers.Start(u.Application, "https://mevius-patch-us.sfo3.digitaloceanspaces.com", u.commiters.Channel, progress, u.errors)
	u.commiters.Start(u.Application, u.errors)

	for _, job := range files {
		u.sourcers.channel <- &RemoteFileSourceJob{
			JobID: uuid.FromStringOrNil(job.Id),
			Path:  job.Path,
		}
	}

	close(u.sourcers.channel)
	u.sourcers.Wait()

	close(u.commiters.Channel)
	u.commiters.Wait()

}
