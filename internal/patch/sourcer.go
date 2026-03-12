package patch

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
)

const remoteHost = "https://mevius-patch-us.sfo3.digitaloceanspaces.com"

// RemoteSourceFileDownloader orchestrates parallel source file downloads
// for a list of manifest files that need a full download.
type RemoteSourceFileDownloader struct {
	application string
	sourcers    *RemoteFileSourceWorkerGroup
	commiters   *FileMetadataCommitWorkerGroup
	errors      chan error
}

func NewRemoteSourceFileDownloader(app string, state *InstallState) *RemoteSourceFileDownloader {
	return &RemoteSourceFileDownloader{
		application: app,
		sourcers:    NewRemoteFileSourceWorkerGroup(8),
		commiters:   NewFileMetadataCommitWorkerGroup(state, 8),
		errors:      make(chan error, 20),
	}
}

// Start downloads all files concurrently and blocks until complete.
// progress receives bytes-written increments for each chunk downloaded.
func (u *RemoteSourceFileDownloader) Start(files []*mevmanifest.File, progress chan<- float32) {
	defer close(progress)

	go func() {
		for err := range u.errors {
			fmt.Printf("[Source Downloader] Error: %v\n", err)
		}
	}()

	u.sourcers.Start(u.application, remoteHost, u.commiters.Channel, progress, u.errors)
	u.commiters.Start(u.application, u.errors)

	for _, f := range files {
		u.sourcers.channel <- &RemoteFileSourceJob{
			Path:     f.Path,
			Checksum: f.Checksum,
			Size:     f.Size,
		}
	}

	close(u.sourcers.channel)
	u.sourcers.Wait()

	close(u.commiters.Channel)
	u.commiters.Wait()

	close(u.errors)
}
