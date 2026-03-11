package orchestrate

import (
	"context"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
)

const (
	StatusUpdateDownloadBundle = "Downloading patch files"
	StatusUpdateUnpackBundle   = "Unpacking patch files"
)

var (
	ErrBundleNotFoundForVersion = func(version patch.Version) error {
		return fmt.Errorf("bundle not found for version %s in manifest", version.String())
	}
)

type BundleProcessStep struct {
	orchestrator *Orchestrator

	ProgressCount float32
	ProgressTotal float32
}

func NewBundleProcessStep(orchestrator *Orchestrator) *BundleProcessStep {
	return &BundleProcessStep{orchestrator: orchestrator}
}

func (s *BundleProcessStep) Run(ctx context.Context, manifest *mevmanifest.Manifest, plan *update.Plan) ([]*patch.RemoteFileMergeJob, error) {

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdateDownloadBundle)

	if manifest.Version == s.orchestrator.CurrentVersion.String() {
		return nil, nil
	}

	var bundle = manifest.BundleForVersion(s.orchestrator.CurrentVersion.String())
	if bundle == nil {
		return nil, ErrBundleNotFoundForVersion(s.orchestrator.CurrentVersion)
	}

	s.orchestrator.ResetSecondaryProgress()
	s.ProgressTotal = float32(bundle.Size)

	var downloaded = make(chan float32, 10)
	go func() {
		for download := range downloaded {
			s.ProgressCount += download
			s.orchestrator.SendSecondaryProgressUpdate(download / s.ProgressTotal)
		}
	}()

	var downloader = update.NewBundleDownloader(s.orchestrator.Application, plan)
	if err := downloader.Download(s.orchestrator.TargetVersion, bundle, downloaded); err != nil {
		return nil, err
	}

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdateUnpackBundle)
	s.orchestrator.ResetSecondaryProgress()
	s.ProgressTotal = float32(len(plan.FilesRequireDownload) + len(plan.FilesRequirePatch))
	s.ProgressCount = 0

	var unpacked = make(chan bool, 10)
	go func() {
		for range unpacked {
			s.ProgressCount++
			s.orchestrator.SendSecondaryProgressUpdate(1 / s.ProgressTotal)
		}
	}()

	results, err := downloader.Unzip(s.orchestrator.CurrentVersion, s.orchestrator.TargetVersion, unpacked)
	if err != nil {
		return nil, err
	}

	return results, nil
}
