package orchestrate

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/update"
)

// RepairStep verifies all installed files against the target manifest and fixes
// anything that is missing or corrupt:
//   - Missing or checksum mismatch → download from target src folder
//   - Exists but needs patching to reach target → apply patch from bundle
//   - Matches target → skip
type RepairStep struct{}

func NewRepairStep() *RepairStep { return &RepairStep{} }

func (s *RepairStep) Run(ctx *Context, o *Orchestrator) error {
	o.SendPrimaryStatusUpdate("Verifying files...")
	o.ResetSecondaryProgress()

	progressTotal := float32(len(ctx.Manifest.Files))
	var progressCount float32

	done := make(chan bool, 64)
	go func() {
		for range done {
			progressCount++
			o.SendSecondaryProgressUpdate(progressCount / progressTotal)
		}
	}()

	planner := update.NewPlanner(ctx.ApplicationName)
	plan := planner.Start(ctx.State, ctx.Manifest, ctx.CurrentVersion.String())
	close(done)

	results := planner.Results()
	o.SendSecondaryStatusUpdate(fmt.Sprintf(
		"%d to fix (%d patch, %d download) of %d total",
		results.TotalPatch+results.TotalDownload,
		results.TotalPatch, results.TotalDownload, results.TotalFiles,
	))

	if results.TotalPatch == 0 && results.TotalDownload == 0 {
		o.SendPrimaryStatusUpdate("All files verified")
		o.SendSecondaryStatusUpdate("No issues found")
		return writeVersion(ctx, o)
	}

	ctx.SourceVersion = ctx.TargetVersion

	if len(plan.FilesRequireDownload) > 0 {
		downloadFiles(ctx, o, plan.FilesRequireDownload, "Repairing missing or corrupt files...")
	}

	if len(plan.FilesRequirePatch) > 0 {
		if err := applyPatches(ctx, o, plan); err != nil {
			return err
		}
	}

	return writeVersion(ctx, o)
}
