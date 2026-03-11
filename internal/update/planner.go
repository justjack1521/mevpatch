package update

import (
	"context"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
)

type UpdateMode int

const (
	UpdateModeDefault    = iota
	UpdateModeRebase     = 1
	UpdateModeForcePatch = 2
	UpdateModeForceFull  = 3
)

type Plan struct {
	Mode                 UpdateMode
	FilesRequireDownload []*mevmanifest.File
	FilesRequirePatch    []*mevmanifest.File
}

type PlannerResult struct {
	TotalFiles    int
	TotalIgnore   int
	TotalPatch    int
	TotalDownload int
}

type Planner struct {
	application string
	collector   *PlanningResultCollector
	validators  *FileCacheValidateWorkerGroup
	errors      chan error
	done        chan<- bool
}

func NewPlanner(app string, done chan<- bool) *Planner {
	return &Planner{
		application: app,
		collector:   NewPlanningResultCollector(),
		validators:  NewFileCacheValidateWorkerGroup(app, 8),
		errors:      make(chan error, 10),
		done:        done,
	}
}

func (p *Planner) Start(state *patch.State, files []*mevmanifest.File, mode UpdateMode) *Plan {

	go func() {
		for err := range p.errors {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	go p.collector.Start()
	p.validators.Start(p.collector.channel, p.errors, p.done)

	for _, f := range files {
		local, _ := state.GetApplicationFile(context.Background(), p.application, f.Path)
		p.validators.channel <- NewFileCacheValidateJob(f, local, mode)
	}

	close(p.validators.channel)
	p.validators.Wait()
	close(p.collector.channel)

	<-p.collector.done

	fmt.Println(fmt.Sprintf("[Total Files Collected] %d", p.collector.Total()))
	fmt.Println(fmt.Sprintf("[Total Files Ignored] %d", p.collector.TotalCategory(FileResultIgnore)))
	fmt.Println(fmt.Sprintf("[Total Files Patching] %d", p.collector.TotalCategory(FileResultPatch)))
	fmt.Println(fmt.Sprintf("[Total Files Downloading] %d", p.collector.TotalCategory(FileResultDownload)))

	close(p.errors)
	close(p.done)

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
