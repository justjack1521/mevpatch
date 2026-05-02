package orchestrate

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/patch"
)

type VersionCheckStep struct{}

func NewVersionCheckStep() *VersionCheckStep { return &VersionCheckStep{} }

func (s *VersionCheckStep) Run(ctx *Context, o *Orchestrator) error {
	o.SendPrimaryStatusUpdate("Checking installed version...")

	state, err := patch.LoadInstallState(ctx.ApplicationName)
	if err != nil {
		return fmt.Errorf("loading install state: %w", err)
	}
	ctx.State = state

	if state.Version == "" {
		ctx.CurrentVersion = patch.Version{}
		o.SendSecondaryStatusUpdate(fmt.Sprintf(
			"Fresh install → %s", ctx.TargetVersion.String(),
		))
		return nil
	}

	current, err := patch.NewVersion(state.Version)
	if err != nil {
		ctx.CurrentVersion = patch.Version{}
		o.SendSecondaryStatusUpdate(fmt.Sprintf(
			"Fresh install → %s", ctx.TargetVersion.String(),
		))
		return nil
	}

	ctx.CurrentVersion = current
	o.SendSecondaryStatusUpdate(fmt.Sprintf(
		"%s → %s", current.String(), ctx.TargetVersion.String(),
	))
	return nil
}
