package orchestrate

import (
	"context"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
)

// Context is the shared state passed between orchestrator steps.
type Context struct {
	context.Context

	ApplicationName string
	CurrentVersion  patch.Version // installed version, zero if fresh install
	TargetVersion   patch.Version
	SourceVersion   patch.Version // rebase version read from manifest

	State    *patch.InstallState
	Manifest *mevmanifest.Manifest
}
