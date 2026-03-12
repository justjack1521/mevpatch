package orchestrate

import (
	"fmt"

	"github.com/justjack1521/mevpatch/internal/patch"
)

// VersionCheckStep loads the local InstallState and determines the currently
// installed version. If no state file exists this is treated as a fresh install.
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
		ctx.CurrentVersion = patch.Version{} // zero → fresh install
		o.SendSecondaryStatusUpdate("No previous installation found")
		return nil
	}

	current, err := patch.NewVersion(state.Version)
	if err != nil {
		// Corrupt version string — treat as fresh install rather than crash.
		fmt.Printf("[Version] Warning: unreadable version %q in state, treating as fresh install\n", state.Version)
		ctx.CurrentVersion = patch.Version{}
		o.SendSecondaryStatusUpdate("No previous installation found")
		return nil
	}

	ctx.CurrentVersion = current
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Installed version: %s", current.String()))
	return nil
}
