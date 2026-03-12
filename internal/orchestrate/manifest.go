package orchestrate

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/manifest"
	"github.com/justjack1521/mevpatch/internal/patch"
)

const remoteHost = "https://mevius-patch-us.sfo3.digitaloceanspaces.com"

// ManifestDownloadStep fetches the manifest for the target version.
// If the target version manifest doesn't exist (e.g. it's x.y.0 and we need
// to rebase), that will be handled by the planner — this step just fetches
// the target version manifest unconditionally.
type ManifestDownloadStep struct{}

func NewManifestDownloadStep() *ManifestDownloadStep { return &ManifestDownloadStep{} }

func (s *ManifestDownloadStep) Run(ctx *Context, o *Orchestrator) error {
	o.SendPrimaryStatusUpdate("Downloading patch manifest...")

	mani, err := manifest.DownloadManifest(remoteHost, ctx.ApplicationName, ctx.TargetVersion)
	if err != nil {
		return fmt.Errorf("downloading manifest for %s: %w", ctx.TargetVersion.String(), err)
	}

	ctx.Manifest = mani
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Manifest loaded for version %s (%d files)", ctx.TargetVersion.String(), len(mani.Files)))
	return nil
}

// RebaseManifestDownloadStep fetches the x.y.0 base manifest when the planner
// determines we need to rebase. Used internally by the planning step.
func downloadMinorBaseManifest(app string, target patch.Version) (*mevmanifest.Manifest, error) {
	base := target.MinorBase()
	mani, err := manifest.DownloadManifest(remoteHost, app, base)
	if err != nil {
		return nil, fmt.Errorf("downloading base manifest %s: %w", base.String(), err)
	}
	return mani, nil
}
