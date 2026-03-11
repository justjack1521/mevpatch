package file

import (
	"context"
	"time"
)

type Repository interface {
	GetApplicationFile(ctx context.Context, application string, path string) (LocalFile, error)
	GetApplicationFiles(ctx context.Context, name string) ([]LocalFile, error)
	CreateApplicationFile(ctx context.Context, app string, path string, size int64, check string, t time.Time) error
}
