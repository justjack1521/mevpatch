package update

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
)

// Plan is the output of the planning phase.
type Plan struct {
	Mode                 UpdateMode
	FilesRequireDownload []*mevmanifest.File
	FilesRequirePatch    []*mevmanifest.File
}

// PlannerResult holds summary counts for UI display.
type PlannerResult struct {
	TotalFiles    int
	TotalIgnore   int
	TotalPatch    int
	TotalDownload int
}

// Planner decides what action is needed for each file in the manifest by
// comparing it against the local InstallState.
type Planner struct {
	application string
	collector   *PlanningResultCollector
	validators  *FileCacheValidateWorkerGroup
	errors      chan error
}

func NewPlanner(app string) *Planner {
	return &Planner{
		application: app,
		collector:   NewPlanningResultCollector(),
		validators:  NewFileCacheValidateWorkerGroup(app, 8),
		errors:      make(chan error, 20),
	}
}

// Start compares the manifest files against state and returns a Plan.
// currentVersion is the installed version string; empty/zero means fresh install.
func (p *Planner) Start(
	state *patch.InstallState,
	manifest *mevmanifest.Manifest,
	currentVersion string,
) *Plan {
	hasBundleForCurrent := manifest.ContainsVersion(currentVersion)
	mode := UpdateModeDefault
	if !hasBundleForCurrent {
		fmt.Printf("[Plan] No bundle for version %q — rebase mode\n", currentVersion)
		mode = UpdateModeRebase
	}

	go func() {
		for err := range p.errors {
			fmt.Printf("[Planner] Error: %v\n", err)
		}
	}()

	go p.collector.Start()
	p.validators.Start(p.collector.channel, p.errors)

	for _, f := range manifest.Files {
		cached, _ := state.Find(f.Path)
		p.validators.channel <- &FileCacheValidateJob{
			File:      f,
			CacheFile: cached,
			Mode:      mode,
			HasBundle: hasBundleForCurrent,
		}
	}

	close(p.validators.channel)
	p.validators.Wait()
	close(p.collector.channel)
	<-p.collector.done

	close(p.errors)

	fmt.Printf("[Plan] Total: %d  Ignore: %d  Patch: %d  Download: %d\n",
		p.collector.Total(),
		p.collector.TotalCategory(FileResultIgnore),
		p.collector.TotalCategory(FileResultPatch),
		p.collector.TotalCategory(FileResultDownload),
	)

	return &Plan{
		Mode:                 mode,
		FilesRequireDownload: p.collector.FilesRequireDownload,
		FilesRequirePatch:    p.collector.FilesRequirePatch,
	}
}

func (p *Planner) Results() PlannerResult {
	return PlannerResult{
		TotalFiles:    p.collector.Total(),
		TotalIgnore:   p.collector.TotalCategory(FileResultIgnore),
		TotalPatch:    p.collector.TotalCategory(FileResultPatch),
		TotalDownload: p.collector.TotalCategory(FileResultDownload),
	}
}
