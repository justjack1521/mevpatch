package orchestrate

// pipeline.go contains shared helpers used by both UpdatePlanningStep and RepairStep.

import (
	"fmt"
	"time"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
)

// downloadFiles downloads a list of source files, driving both progress bars:
//   - Overall bar: one tick per completed file
//   - Files bar: total bytes received across all files
func downloadFiles(ctx *Context, o *Orchestrator, files []*mevmanifest.File, label string) {
	o.SendPrimaryStatusUpdate(label)
	o.ResetPrimaryProgress()
	o.ResetSecondaryProgress()

	totalFiles := float32(len(files))
	var completedFiles float32

	var totalBytes float64
	for _, f := range files {
		totalBytes += float64(f.Size)
	}
	var receivedBytes float64

	o.SendSecondaryStatusUpdate(fmt.Sprintf("%d files  •  %.1f MB", len(files), totalBytes/1024/1024))

	progress := make(chan patch.SourceProgress, 128)

	go func() {
		for p := range progress {
			switch {
			case p.FileDone:
				completedFiles++
				o.SetPrimaryProgress(completedFiles / totalFiles)
			case p.BytesRead > 0:
				receivedBytes += float64(p.BytesRead)
				if totalBytes > 0 {
					o.SetSecondaryProgress(float32(receivedBytes / totalBytes))
				}
			}
		}
	}()

	downloader := patch.NewRemoteSourceFileDownloader(ctx.ApplicationName, ctx.SourceVersion.String(), ctx.State)
	downloader.Start(files, progress)
}

// applyPatches downloads the patch bundle for ctx.CurrentVersion and applies it.
func applyPatches(ctx *Context, o *Orchestrator, plan *update.Plan) error {
	bundle := ctx.Manifest.BundleForVersion(ctx.CurrentVersion.String())
	if bundle == nil {
		return fmt.Errorf("no patch bundle in manifest for version %s", ctx.CurrentVersion.String())
	}

	o.SendPrimaryStatusUpdate(fmt.Sprintf("Downloading patch bundle for %s...", ctx.CurrentVersion.String()))
	o.ResetSecondaryProgress()

	bundleProgress := make(chan float32, 64)
	var bundleReceived float32
	bundleTotal := float32(bundle.Size)
	go func() {
		for n := range bundleProgress {
			bundleReceived += n
			if bundleTotal > 0 {
				o.SendSecondaryProgressUpdate(bundleReceived / bundleTotal)
			}
		}
	}()

	bundleDownloader := update.NewBundleDownloader(ctx.ApplicationName, plan)
	if err := bundleDownloader.Download(ctx.TargetVersion, bundle, bundleProgress); err != nil {
		return fmt.Errorf("downloading patch bundle: %w", err)
	}

	o.SendPrimaryStatusUpdate("Applying patches...")
	o.ResetSecondaryProgress()

	unpacked := make(chan bool, 64)
	patchTotal := float32(len(plan.FilesRequirePatch))
	var patchCount float32
	go func() {
		for range unpacked {
			patchCount++
			o.SendSecondaryProgressUpdate(patchCount / patchTotal)
		}
	}()

	jobs, err := bundleDownloader.Unzip(ctx.CurrentVersion, ctx.TargetVersion, unpacked)
	if err != nil {
		return fmt.Errorf("extracting patch bundle: %w", err)
	}

	merger := patch.NewRemotePatchFileMerger(ctx.ApplicationName, o.Merger, ctx.State)
	merger.Start(jobs)

	o.SendPrimaryStatusUpdate("Patches applied")
	o.SendSecondaryStatusUpdate(fmt.Sprintf("%d files patched successfully", len(jobs)))
	o.ResetSecondaryProgress()

	return nil
}

// writeVersion persists the updated version string to the state file.
func writeVersion(ctx *Context, o *Orchestrator) error {
	ctx.State.Version = ctx.TargetVersion.String()
	ctx.State.UpdatedAt = time.Now().UTC()
	if err := patch.SaveInstallState(ctx.ApplicationName, ctx.State); err != nil {
		return fmt.Errorf("saving install state: %w", err)
	}
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Version updated to %s", ctx.TargetVersion.String()))
	return nil
}
