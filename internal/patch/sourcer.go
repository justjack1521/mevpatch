package patch

import (
	"fmt"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
)

const remoteHost = "https://patch.blankproject.dev"

// RemoteSourceFileDownloader orchestrates parallel source file downloads.
type RemoteSourceFileDownloader struct {
	application string
	version     string
	sourcers    *RemoteFileSourceWorkerGroup
	commiters   *FileMetadataCommitWorkerGroup
	errors      chan error
}

func NewRemoteSourceFileDownloader(app string, version string, state *InstallState) *RemoteSourceFileDownloader {
	return &RemoteSourceFileDownloader{
		application: app,
		version:     version,
		sourcers:    NewRemoteFileSourceWorkerGroup(8),
		commiters:   NewFileMetadataCommitWorkerGroup(state, 8),
		errors:      make(chan error, 20),
	}
}

// Start downloads all files concurrently and blocks until complete.
// progress receives a SourceProgress per chunk/file for UI updates.
func (u *RemoteSourceFileDownloader) Start(files []*mevmanifest.File, progress chan<- SourceProgress) {
	go func() {
		for err := range u.errors {
			fmt.Printf("[Source] Error: %v\n", err)
		}
	}()

	u.sourcers.Start(u.application, remoteHost, u.commiters.Channel, progress, u.errors)
	u.commiters.Start(u.application, u.errors)

	for _, f := range files {
		u.sourcers.channel <- &RemoteFileSourceJob{
			Path:          f.Path,
			Checksum:      f.Checksum,
			Size:          f.Size,
			SourceVersion: u.version,
		}
	}

	close(u.sourcers.channel)
	u.sourcers.Wait()

	close(u.commiters.Channel)
	u.commiters.Wait()

	close(u.errors)
	close(progress)
}
