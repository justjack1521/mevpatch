package patch

import (
	"context"
)

type Repository interface {
	GetApplicationVersion(ctx context.Context, name string) (Version, error)
	UpdateApplicationVersion(ctx context.Context, name string, version Version) error
}
