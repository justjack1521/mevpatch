package orchestrate

import (
	"fmt"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
)

// UpdatePlanningStep compares local state against the target manifest and
// decides what action is needed for each file.
//
// Logic:
//  1. If the manifest has a bundle for the current installed version → patch mode.
//     Files with differing checksum will be patched from the bundle.
//  2. If there is no bundle for the current version (too old or fresh install) → rebase mode.
//     - Fresh install: download all files from source directly.
//     - Version outside patch window: download the x.y.0 base files, re-plan
//     patches from base → target, then apply.  More efficient than full download
//     when most files overlap between minor versions.
type UpdatePlanningStep struct {
	progressCount float32
	progressTotal float32
}

func NewUpdatePlanningStep() *UpdatePlanningStep { return &UpdatePlanningStep{} }

func (s *UpdatePlanningStep) Run(ctx *Context, o *Orchestrator) error {
	o.SendPrimaryStatusUpdate("Planning update...")
	o.ResetSecondaryProgress()

	s.progressTotal = float32(len(ctx.Manifest.Files))
	s.progressCount = 0

	done := make(chan bool, 64)
	go func() {
		for range done {
			s.progressCount++
			o.SendSecondaryProgressUpdate(s.progressCount / s.progressTotal)
		}
	}()

	planner := update.NewPlanner(ctx.ApplicationName)
	plan := planner.Start(ctx.State, ctx.Manifest, ctx.CurrentVersion.String())
	close(done)

	results := planner.Results()
	o.SendSecondaryStatusUpdate(fmt.Sprintf(
		"%d to update (%d patch, %d download) of %d total",
		results.TotalPatch+results.TotalDownload,
		results.TotalPatch, results.TotalDownload, results.TotalFiles,
	))

	if results.TotalPatch == 0 && results.TotalDownload == 0 {
		return s.writeVersion(ctx, o)
	}

	// ── Source download phase ─────────────────────────────────────────────
	if len(plan.FilesRequireDownload) > 0 {
		if plan.Mode == update.UpdateModeRebase && !ctx.CurrentVersion.Zero() {
			// We have an install but it's outside the patch window.
			// Download the x.y.0 base for the affected files, then patch up.
			if err := s.rebaseDownload(ctx, o, plan); err != nil {
				return err
			}
		} else {
			// Fresh install or individual missing/corrupt files.
			if err := s.sourceDownload(ctx, o, plan); err != nil {
				return err
			}
		}
	}

	// ── Patch phase ───────────────────────────────────────────────────────
	if len(plan.FilesRequirePatch) > 0 {
		if err := s.applyPatches(ctx, o, plan); err != nil {
			return err
		}
	}

	return s.writeVersion(ctx, o)
}

// sourceDownload downloads full source files for all entries in FilesRequireDownload.
func (s *UpdatePlanningStep) sourceDownload(ctx *Context, o *Orchestrator, plan *update.Plan) error {
	o.SendPrimaryStatusUpdate("Downloading files from source...")
	o.ResetSecondaryProgress()

	var totalBytes float32
	for _, f := range plan.FilesRequireDownload {
		totalBytes += float32(f.Size)
	}
	o.SendSecondaryStatusUpdate(fmt.Sprintf("%.1f MB to download", float64(totalBytes)/1024/1024))

	progress := make(chan float32, 64)
	var received float32
	go func() {
		for n := range progress {
			received += n
			if totalBytes > 0 {
				o.SendSecondaryProgressUpdate(received / totalBytes)
			}
		}
	}()

	downloader := patch.NewRemoteSourceFileDownloader(ctx.ApplicationName, ctx.State)
	downloader.Start(plan.FilesRequireDownload, progress)
	return nil
}

// rebaseDownload downloads x.y.0 base files and re-plans patches from there.
func (s *UpdatePlanningStep) rebaseDownload(ctx *Context, o *Orchestrator, plan *update.Plan) error {
	base := ctx.TargetVersion.MinorBase()
	o.SendPrimaryStatusUpdate(fmt.Sprintf("Downloading base version %s...", base.String()))
	o.ResetSecondaryProgress()

	// Fetch the base (x.y.0) manifest.
	baseMani, err := downloadMinorBaseManifest(ctx.ApplicationName, ctx.TargetVersion)
	if err != nil {
		// Base manifest missing (target IS the base) → fall back to direct source download.
		fmt.Printf("[Planner] Base manifest unavailable (%v), downloading source directly\n", err)
		return s.sourceDownload(ctx, o, plan)
	}

	// Build lookup of paths that need downloading.
	needsDownload := make(map[string]bool, len(plan.FilesRequireDownload))
	for _, f := range plan.FilesRequireDownload {
		needsDownload[f.Path] = true
	}

	// Filter base manifest to only the files we actually need.
	var baseFiles []*mevmanifest.File
	for _, f := range baseMani.Files {
		if needsDownload[f.Path] {
			baseFiles = append(baseFiles, f)
		}
	}
	if len(baseFiles) == 0 {
		return s.sourceDownload(ctx, o, plan)
	}

	var totalBytes float32
	for _, f := range baseFiles {
		totalBytes += float32(f.Size)
	}
	o.SendSecondaryStatusUpdate(fmt.Sprintf("%.1f MB base files to download", float64(totalBytes)/1024/1024))

	progress := make(chan float32, 64)
	var received float32
	go func() {
		for n := range progress {
			received += n
			if totalBytes > 0 {
				o.SendSecondaryProgressUpdate(received / totalBytes)
			}
		}
	}()

	downloader := patch.NewRemoteSourceFileDownloader(ctx.ApplicationName, ctx.State)
	downloader.Start(baseFiles, progress)

	// Update ctx.CurrentVersion so applyPatches knows which bundle to use.
	ctx.CurrentVersion = base

	// Re-plan from the base version to the target.
	o.SendPrimaryStatusUpdate(fmt.Sprintf("Re-planning patches %s → %s...", base.String(), ctx.TargetVersion.String()))
	replanner := update.NewPlanner(ctx.ApplicationName)
	newPlan := replanner.Start(ctx.State, ctx.Manifest, base.String())

	plan.FilesRequirePatch = newPlan.FilesRequirePatch
	plan.FilesRequireDownload = newPlan.FilesRequireDownload
	plan.Mode = newPlan.Mode

	// Anything still needing a direct download after re-planning.
	if len(plan.FilesRequireDownload) > 0 {
		if err := s.sourceDownload(ctx, o, plan); err != nil {
			return err
		}
	}

	return nil
}

// applyPatches downloads the bundle for ctx.CurrentVersion and applies all patches.
func (s *UpdatePlanningStep) applyPatches(ctx *Context, o *Orchestrator, plan *update.Plan) error {
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

	return nil
}

// writeVersion persists the updated version string to the state file.
func (s *UpdatePlanningStep) writeVersion(ctx *Context, o *Orchestrator) error {
	ctx.State.Version = ctx.TargetVersion.String()
	if err := patch.SaveInstallState(ctx.ApplicationName, ctx.State); err != nil {
		return fmt.Errorf("saving install state: %w", err)
	}
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Version updated to %s", ctx.TargetVersion.String()))
	return nil
}
