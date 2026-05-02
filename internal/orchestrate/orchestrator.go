package orchestrate

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/justjack1521/mevpatch/internal/file"
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
	Context       *Context
	Arguments     OrchestratorArguments
	Merger        *patch.MergeTool
	UpdateChannel chan gui.PatchUpdate
	Steps         []OrchestratorStep
	Logger        *slog.Logger
}

type OrchestratorArguments struct {
	Application          string
	TargetVersion        patch.Version
	LegacyConnectionMode bool
	Debug                bool
	LogFile              *os.File
}

func NewOrchestrator(args OrchestratorArguments, merger *patch.MergeTool, updates chan gui.PatchUpdate, steps []OrchestratorStep) *Orchestrator {
	return &Orchestrator{
		Arguments:     args,
		Merger:        merger,
		UpdateChannel: updates,
		Steps:         steps,
	}
}

func (o *Orchestrator) CreateLogger() *slog.Logger {
	if o.Arguments.Debug {
		log, err := file.OpenLogFile()
		if err == nil {
			logger := slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
			slog.SetDefault(logger)
			os.Stdout = log
			os.Stderr = log
			return logger
		}
	}
	null, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	logger := slog.New(slog.NewTextHandler(null, nil))
	slog.SetDefault(logger)
	return logger
}

// Start runs all steps in sequence. Designed to be called in a goroutine.
func (o *Orchestrator) Start(ctx context.Context) {

	defer func() {
		if r := recover(); r != nil {
			o.SendError(fmt.Errorf("unexpected error: %v", r))
		}
		close(o.UpdateChannel)
	}()

	o.Logger = o.CreateLogger()

	if o.Arguments.LegacyConnectionMode {
		patch.ForceHTTP1Client()
		o.SendPrimaryStatusUpdate("Switching to legacy connection mode...")
		o.ResetSecondaryProgress()
	}

	o.ResetPrimaryProgress()
	o.ResetSecondaryProgress()

	o.Context = &Context{
		Context:         ctx,
		ApplicationName: o.Arguments.Application,
		TargetVersion:   o.Arguments.TargetVersion,
	}

	for _, step := range o.Steps {
		if err := step.Run(o.Context, o); err != nil {
			o.SendError(err)
			return
		}
	}

	o.SendPrimaryStatusUpdate("Up to date!")
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Version %s", o.Arguments.TargetVersion.String()))
}

// ── GUI helpers ───────────────────────────────────────────────────────────────

func (o *Orchestrator) SendPrimaryStatusUpdate(value string) {
	o.Logger.Log(o.Context, slog.LevelInfo, fmt.Sprintf("[PRIMARY] %s", value))
	o.UpdateChannel <- gui.StatusUpdate{Primary: value}
}

func (o *Orchestrator) SendSecondaryStatusUpdate(value string) {
	o.Logger.Log(o.Context, slog.LevelInfo, fmt.Sprintf("[SECONDARY] %s", value))
	o.UpdateChannel <- gui.StatusUpdate{Secondary: value}
}

func (o *Orchestrator) ResetPrimaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Reset: true}
}

func (o *Orchestrator) SendPrimaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Value: value}
}

func (o *Orchestrator) SetPrimaryProgress(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypePrimary, Set: true, Value: value}
}

func (o *Orchestrator) ResetSecondaryProgress() {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Reset: true}
}

func (o *Orchestrator) SendSecondaryProgressUpdate(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Value: value}
}

func (o *Orchestrator) SetSecondaryProgress(value float32) {
	o.UpdateChannel <- gui.ProgressUpdate{ProgressUpdateType: gui.ProgressUpdateTypeSecondary, Set: true, Value: value}
}

func (o *Orchestrator) SendError(err error) {
	o.Logger.Log(o.Context, slog.LevelError, fmt.Sprintf("[ERROR] %s", err.Error()))
	o.UpdateChannel <- gui.ErrorUpdate{Value: err}
}

func (o *Orchestrator) SendLog(format string, args ...any) {
	var value = fmt.Sprintf(format, args...)
	o.Logger.Log(o.Context, slog.LevelDebug, fmt.Sprintf("[LOG] %s", value))
	o.UpdateChannel <- gui.LogUpdate{Value: value}
}
