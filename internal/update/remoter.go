package update

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
)

type RemoteFileValidator struct {
	application string
	validators  *FileCacheValidateWorkerGroup
	collectors  chan *FileCategorisationResult
	errors      chan error
}

func NewRemoteFileValidator(app string, repo file.Repository, count int) *RemoteFileValidator {
	var updater = &RemoteFileValidator{
		application: app,
		validators:  NewFileCacheValidateWorkerGroup(app, count),
		errors:      make(chan error, 10),
	}
	return updater
}

func (u *RemoteFileValidator) Start(validates []*FileCacheValidateJob) {

	go func() {
		for err := range u.errors {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	//u.validators.Start(u.collectors, u.errors)
	//u.sourcers.Start(u.application, "https://mevius-patch-us.sfo3.digitaloceanspaces.com", u.committers.channel, u.errors)
	//u.committers.Start(u.errors)
	//
	//for _, job := range validates {
	//	u.validators.channel <- &FileCacheValidateJob{
	//		Path:         job.Path,
	//		Size:         job.Size,
	//		Checksum:     job.Checksum,
	//		DownloadPath: job.DownloadPath,
	//	}
	//}
	//close(u.validators.channel)
	//
	//u.validators.Wait()
	//close(u.sourcers.channel)
	//u.sourcers.Wait()
	//close(u.committers.channel)
	//u.committers.Wait()
	//close(u.errors)

}
