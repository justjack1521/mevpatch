package orchestrate

import (
	"fmt"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/manifest"
	"github.com/justjack1521/mevpatch/internal/patch"
)

const remoteHost = "https://patch.blankproject.dev"

type ManifestDownloadStep struct{}

func NewManifestDownloadStep() *ManifestDownloadStep { return &ManifestDownloadStep{} }

func (s *ManifestDownloadStep) Run(ctx *Context, o *Orchestrator) error {
	o.SendPrimaryStatusUpdate("Downloading patch manifest...")

	mani, err := manifest.DownloadManifest(remoteHost, ctx.ApplicationName, ctx.TargetVersion)
	if err != nil {
		return fmt.Errorf("downloading manifest for %s: %w", ctx.TargetVersion.String(), err)
	}
	ctx.Manifest = mani

	ctx.SourceVersion = sourceVersionFromManifest(mani, ctx.TargetVersion)

	o.SendSecondaryStatusUpdate(fmt.Sprintf(
		"Manifest loaded for %s (%d files, rebase %s)",
		ctx.TargetVersion.String(), len(mani.Files), ctx.SourceVersion.String(),
	))
	return nil
}

// sourceVersionFromManifest reads the Rebase field from the manifest.
// Falls back to MinorBase() if the field is empty or unparseable.
func sourceVersionFromManifest(mani *mevmanifest.Manifest, target patch.Version) patch.Version {
	if mani.Rebase == "" {
		return target.MinorBase()
	}
	v, err := patch.NewVersion(mani.Rebase)
	if err != nil {
		return target.MinorBase()
	}
	return v
}

// downloadManifest fetches the manifest for a specific version.
func downloadManifest(app string, version patch.Version) (*mevmanifest.Manifest, error) {
	mani, err := manifest.DownloadManifest(remoteHost, app, version)
	if err != nil {
		return nil, fmt.Errorf("downloading manifest %s: %w", version.String(), err)
	}
	return mani, nil
}

func downloadMinorBaseManifest(app string, ctx *Context) (*mevmanifest.Manifest, error) {
	mani, err := manifest.DownloadManifest(remoteHost, app, ctx.SourceVersion)
	if err != nil {
		return nil, fmt.Errorf("downloading base manifest %s: %w", ctx.SourceVersion.String(), err)
	}
	return mani, nil
}
