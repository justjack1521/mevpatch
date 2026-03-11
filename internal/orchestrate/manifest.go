package orchestrate

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/manifest"
	"github.com/justjack1521/mevpatch/internal/patch"
)

const (
	PrimaryStatusUpdateDownloadManifest = "Downloading patch manifest"
)

var (
	SecondaryStatusUpdateManifestFound = func(version patch.Version) string {
		return fmt.Sprintf("Manifest found for version %s", version.String())
	}
)

var (
	ErrFailedToDownloadManifest = func(err error) error {
		return fmt.Errorf("failed to download patch manifest: %w", err)
	}
)

type ManifestDownloadStep struct {
	orchestrator *Orchestrator
}

func NewManifestDownloadStep() *ManifestDownloadStep {
	return &ManifestDownloadStep{}
}

func (s *ManifestDownloadStep) Run(ctx *Context, orchestrator *Orchestrator) error {

	orchestrator.SendPrimaryStatusUpdate(PrimaryStatusUpdateDownloadManifest)

	mani, err := manifest.DownloadManifest("https://mevius-patch-us.sfo3.digitaloceanspaces.com", ctx.ApplicationName, ctx.TargetVersion)
	if err != nil {
		return ErrFailedToDownloadManifest(err)
	}

	orchestrator.SendSecondaryStatusUpdate(SecondaryStatusUpdateManifestFound(ctx.TargetVersion))

	ctx.Manifest = mani

	return nil

}
