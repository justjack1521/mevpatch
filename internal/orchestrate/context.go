package orchestrate

import (
	"context"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
)

type Context struct {
	context.Context
	ApplicationName string
	CurrentVersion  patch.Version
	TargetVersion   patch.Version
	LocalState      *patch.State
	Manifest        *mevmanifest.Manifest
}
