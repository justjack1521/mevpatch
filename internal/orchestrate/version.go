package orchestrate

import (
	"encoding/json"
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"os"
	"path/filepath"
)

const (
	PrimaryStatusUpdateCheckingVersion = "Checking current version"
)

const (
	SecondaryStatusUpdateNoVersionFound = "No installed version found"
)

var (
	SecondaryStatusUpdateCurrentVersionFound = func(version patch.Version) string {
		return fmt.Sprintf("Current version found as %s", version.String())
	}
	SecondaryStatusUpdateInvalidVersionFound = func(version string) string {
		return fmt.Sprintf("Invalid version found as %s", version)
	}
)

type VersionCheckStep struct {
}

func NewVersionCheckStep() *VersionCheckStep {
	return &VersionCheckStep{}
}

func (s *VersionCheckStep) Run(ctx *Context, orchestrator *Orchestrator) error {

	orchestrator.SendPrimaryStatusUpdate(PrimaryStatusUpdateCheckingVersion)

	patcher, err := file.PatcherPath()
	if err != nil {
		return err
	}
	var path = filepath.Join(patcher, fmt.Sprintf("%s.json", ctx.ApplicationName))

	if err := file.ExistsAtPath(path); err != nil {
		ctx.CurrentVersion = patch.Version{Major: 1, Minor: 0, Patch: 0}
		orchestrator.SendSecondaryStatusUpdate(SecondaryStatusUpdateNoVersionFound)
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var state = &patch.State{}
	var decoder = json.NewDecoder(f)
	if err := decoder.Decode(state); err != nil {
		return err
	}

	current, err := patch.NewVersion(state.LocalHash)
	if err != nil {
		ctx.CurrentVersion = patch.Version{Major: 1, Minor: 0, Patch: 0}
		orchestrator.SendSecondaryStatusUpdate(SecondaryStatusUpdateNoVersionFound)
		return nil
	}

	ctx.CurrentVersion = current
	ctx.LocalState = state
	orchestrator.SendSecondaryStatusUpdate(SecondaryStatusUpdateCurrentVersionFound(ctx.CurrentVersion))

	return nil

}
