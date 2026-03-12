package orchestrate

import (
	"context"
	"fmt"

	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/patch"
)

// OrchestratorStep is one stage of the patching pipeline.
type OrchestratorStep interface {
	Run(ctx *Context, o *Orchestrator) error
}

// Orchestrator runs a series of steps in sequence, broadcasting progress
// updates to the GUI via UpdateChannel.
type Orchestrator struct {
	Application   string
	TargetVersion patch.Version
	Merger        *patch.MergeTool
	UpdateChannel chan gui.PatchUpdate
	Steps         []OrchestratorStep
}

func NewOrchestrator(
	app string,
	target patch.Version,
	merger *patch.MergeTool,
	updates chan gui.PatchUpdate,
	steps []OrchestratorStep,
) *Orchestrator {
	return &Orchestrator{
		Application:   app,
		TargetVersion: target,
		Merger:        merger,
		UpdateChannel: updates,
		Steps:         steps,
	}
}

// Start runs all steps in sequence. Designed to be called in a goroutine.
func (o *Orchestrator) Start(ctx context.Context) {
	o.ResetPrimaryProgress()
	o.ResetSecondaryProgress()

	octx := &Context{
		Context:         ctx,
		ApplicationName: o.Application,
		TargetVersion:   o.TargetVersion,
	}

	total := len(o.Steps)
	for i, step := range o.Steps {
		if err := step.Run(octx, o); err != nil {
			o.SendError(err)
			return
		}
		o.SendPrimaryProgressUpdate(float32(i+1) / float32(total))
	}

	o.SendPrimaryStatusUpdate("Up to date!")
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Version %s", o.TargetVersion.String()))
}

// ── GUI helpers ───────────────────────────────────────────────────────────────

func (o *Orchestrator) SendPrimaryStatusUpdate(value string) {
	fmt.Printf("[Status] %s\n", value)
	o.UpdateChannel <- gui.StatusUpdate{Primary: value}
}

func (o *Orchestrator) SendSecondaryStatusUpdate(value string) {
	fmt.Printf("[Status] %s\n", value)
	o.UpdateChannel <- gui.StatusUpdate{Secondary: value}
}

func (o *Orchestrator) ResetPrimaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Reset: true}
}

func (o *Orchestrator) SendPrimaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Value: value}
}

func (o *Orchestrator) ResetSecondaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Reset: true}
}

func (o *Orchestrator) SendSecondaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Value: value}
}

func (o *Orchestrator) SendError(err error) {
	fmt.Printf("[Error] %v\n", err)
	o.UpdateChannel <- gui.ErrorUpdate{Value: err}
}
