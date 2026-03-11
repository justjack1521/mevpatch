package orchestrate

import (
	"context"
	"fmt"
	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/patch"
)

type Orchestrator struct {
	Application    string
	CurrentVersion patch.Version
	TargetVersion  patch.Version
	Merger         *patch.MergeTool
	UpdateChannel  chan gui.PatchUpdate
	Steps          []OrchestratorStep
}

func NewOrchestrator(application string, target patch.Version, updates chan gui.PatchUpdate, steps []OrchestratorStep) *Orchestrator {
	return &Orchestrator{
		Application:   application,
		TargetVersion: target,
		UpdateChannel: updates,
		Steps:         steps,
	}
}

type OrchestratorStep interface {
	Run(ctx *Context, orchestrator *Orchestrator) error
}

func (o *Orchestrator) Start(ctx context.Context) {

	o.ResetPrimaryProgress()
	o.ResetSecondaryProgress()

	var octx = &Context{
		Context:         ctx,
		ApplicationName: o.Application,
		TargetVersion:   o.TargetVersion,
	}

	var total = len(o.Steps)
	var idx = 0

	for _, step := range o.Steps {
		idx++
		if err := step.Run(octx, o); err != nil {
			o.SendError(err)
			return
		}
		o.SendPrimaryProgressUpdate(float32(idx) / float32(total))
	}

	return

}

//func (o *Orchestrator) Run(ctx context.Context) {
//
//	var octx = &Context{
//		Context:         ctx,
//		ApplicationName: o.Application,
//		TargetVersion:   o.TargetVersion,
//	}
//
//	var dbs = NewDatabaseInitialiseStep(o)
//	repository, err := dbs.Run(ctx)
//	if err != nil {
//		o.SendError(err)
//		return
//	}
//	defer func(repository *database.PatchingRepository) {
//		err := repository.Close()
//		if err != nil {
//			o.SendError(err)
//		}
//	}(repository)
//
//	var vcs = NewVersionCheckStep(o)
//	if err := vcs.Run(octx); err != nil {
//		o.SendError(err)
//		return
//	}
//
//	var mds = NewManifestDownloadStep(o)
//	if err := mds.Run(octx); err != nil {
//		o.SendError(err)
//		return
//	}
//
//	var ups = NewUpdatePlanningStep(o, repository)
//	var plan = ups.Run(octx)
//
//	var rsd = NewSourceDownloadStep(o, repository)
//	rsd.Run(ctx, plan)
//
//	var bds = NewBundleProcessStep(o)
//	jobs, err := bds.Run(ctx, octx.Manifest, plan)
//	if err != nil {
//		o.SendError(err)
//		return
//	}
//
//	var pms = NewPatchMergeStep(o, repository)
//	if err := pms.Run(ctx, jobs); err != nil {
//		o.SendError(err)
//		return
//	}
//
//	o.SendPrimaryStatusUpdate("Finishing up")
//
//	if err := repository.UpdateApplicationVersion(ctx, o.Application, o.TargetVersion); err != nil {
//		o.SendError(err)
//		return
//	}
//
//}

func (o *Orchestrator) SendPrimaryStatusUpdate(value string) {
	fmt.Println(fmt.Sprintf("Orchestrator primary update: %s", value))
	o.UpdateChannel <- gui.StatusUpdate{Primary: value}
}

func (o *Orchestrator) SendSecondaryStatusUpdate(value string) {
	fmt.Println(fmt.Sprintf("Orchestrator secondary update: %s", value))
	o.UpdateChannel <- gui.StatusUpdate{Secondary: value}
}

func (o *Orchestrator) ResetPrimaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Reset: true}
}

func (o *Orchestrator) SendPrimaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{
		ProgressUpdateType: gui.ProgressUpdateTypePrimary,
		Value:              value,
	}
}

func (o *Orchestrator) ResetSecondaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Reset: true}
}

func (o *Orchestrator) SendSecondaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{
		ProgressUpdateType: gui.ProgressUpdateTypeSecondary,
		Value:              value,
	}
}

func (o *Orchestrator) SendError(err error) {
	fmt.Println(fmt.Sprintf("Orhcestrator error: %s", err))
	o.UpdateChannel <- gui.ErrorUpdate{Value: err}
}
