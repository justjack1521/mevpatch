package orchestrate

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
)

const (
	StatusUpdatePlanningUpdates = "Planning file updates"
)

type UpdatePlanningStep struct {
	orchestrator *Orchestrator
	repository   file.Repository

	ProgressCount float32
	ProgressTotal float32

	UpdateMode update.UpdateMode
}

func NewUpdatePlanningStep(orchestrator *Orchestrator, repository file.Repository) *UpdatePlanningStep {
	return &UpdatePlanningStep{orchestrator: orchestrator, repository: repository}
}

func (s *UpdatePlanningStep) Run(ctx *Context) *update.Plan {

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdatePlanningUpdates)
	s.orchestrator.ResetSecondaryProgress()
	s.ProgressCount = 0
	s.ProgressTotal = float32(len(ctx.Manifest.Files))

	var mode update.UpdateMode
	if ctx.Manifest.ContainsVersion(s.orchestrator.CurrentVersion.String()) == false {
		ctx.CurrentVersion = patch.Version{
			Major: ctx.TargetVersion.Major,
			Minor: ctx.TargetVersion.Minor,
			Patch: 0,
		}
		mode = update.UpdateModeRebase
	}

	var done = make(chan bool, 10)
	go func() {
		for range done {
			s.ProgressCount++
			s.orchestrator.SendSecondaryProgressUpdate(s.ProgressCount / s.ProgressTotal)
		}
	}()

	var planner = update.NewPlanner(s.orchestrator.Application, done)
	var plan = planner.Start(ctx.LocalState, ctx.Manifest.Files, mode)

	var results = planner.Results()
	var secondary = fmt.Sprintf("Files to patch: %d / %d", results.TotalFiles-results.TotalIgnore, results.TotalFiles)
	s.orchestrator.SendSecondaryStatusUpdate(secondary)

	return plan

}
