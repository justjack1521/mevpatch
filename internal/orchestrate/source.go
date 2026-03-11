package orchestrate

import (
	"context"
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
)

const StatusUpdateDownloadSource = "Downloading files from source"

type SourceDownloadStep struct {
	orchestrator *Orchestrator
	repository   file.Repository

	ProgressCount float32
	ProgressTotal float32
}

func NewSourceDownloadStep(orchestrator *Orchestrator, repository file.Repository) *SourceDownloadStep {
	return &SourceDownloadStep{orchestrator: orchestrator, repository: repository}
}

func (s *SourceDownloadStep) Run(ctx context.Context, plan *update.Plan) {

	if len(plan.FilesRequireDownload) == 0 {
		return
	}

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdateDownloadSource)

	s.orchestrator.ResetSecondaryProgress()
	s.ProgressCount = 0

	for _, f := range plan.FilesRequireDownload {
		s.ProgressTotal += float32(f.Size)
	}

	var mb = float64(s.ProgressTotal) / (1024 * 1024)
	s.orchestrator.SendSecondaryStatusUpdate(fmt.Sprintf("Total size to download: %.2f MB\n", mb))

	var downloaded = make(chan float32, 10)
	go func() {
		for download := range downloaded {
			s.ProgressCount += download
			s.orchestrator.SendSecondaryProgressUpdate(download / s.ProgressTotal)
		}
	}()

	var downloader = patch.NewRemoteSourceFileDownloader(s.orchestrator.Application, s.repository)
	downloader.Start(plan.FilesRequireDownload, downloaded)

}
