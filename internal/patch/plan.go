package patch

import mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"

type Plan struct {
	Bundle      *mevmanifest.Bundle
	RemoteFiles []*mevmanifest.File
}
