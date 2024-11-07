package database

import (
	"context"
	"database/sql"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/gensql"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"sync"
	"time"
)

type PatchingRepository struct {
	queries *mevmanifest.Queries
	wmx     sync.Mutex
}

func NewPatchingRepository(db *sql.DB) *PatchingRepository {
	return &PatchingRepository{
		queries: mevmanifest.New(db),
	}
}

func (r *PatchingRepository) GetApplicationVersion(ctx context.Context, name string) (patch.Version, error) {
	result, err := r.queries.GetApplicationVersion(ctx, name)
	if err != nil {
		return patch.Version{}, err
	}
	return patch.Version{
		Major: int(result.Major),
		Minor: int(result.Minor),
		Patch: int(result.Patch),
	}, nil
}

func (r *PatchingRepository) GetApplicationFile(ctx context.Context, application string, path string) (file.LocalFile, error) {
	result, err := r.queries.GetApplicationFile(ctx, mevmanifest.GetApplicationFileParams{
		Path:        path,
		Application: application,
	})
	if err != nil {
		return file.LocalFile{}, err
	}
	return file.LocalFile{
		Path:      result.Path,
		Size:      result.Size,
		Checksum:  result.Checksum,
		Timestamp: time.Unix(result.Timestamp, 0),
	}, nil

}

func (r *PatchingRepository) GetApplicationFiles(ctx context.Context, name string) ([]file.LocalFile, error) {
	results, err := r.queries.GetApplicationFiles(ctx, name)
	if err != nil {
		return nil, err
	}
	var dest = make([]file.LocalFile, len(results))
	for index, value := range results {
		dest[index] = file.LocalFile{
			Path:      value.Path,
			Size:      value.Size,
			Checksum:  value.Checksum,
			Timestamp: time.Unix(value.Timestamp, 0),
		}
	}
	return dest, nil
}

func (r *PatchingRepository) CreateApplicationFile(ctx context.Context, app string, path string, size int64, check string, t time.Time) error {
	r.wmx.Lock()
	defer r.wmx.Unlock()
	var args = mevmanifest.CreateApplicationFileParams{
		Path:        path,
		Size:        size,
		Checksum:    check,
		Timestamp:   t.Unix(),
		Application: app,
	}
	return r.queries.CreateApplicationFile(ctx, args)
}
