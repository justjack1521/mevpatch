package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/gensql"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"sync"
	"time"
)

type PatchingRepository struct {
	database *sql.DB
	queries  *mevmanifest.Queries
	wmx      sync.Mutex
}

func NewPatchingRepository(db *sql.DB) *PatchingRepository {
	return &PatchingRepository{
		database: db,
		queries:  mevmanifest.New(db),
	}
}

func (r *PatchingRepository) Close() error {
	_, err := r.database.Exec("PRAGMA journal_mode = DELETE;")
	if err != nil {
		return fmt.Errorf("failed to reset journal mode to DELETE: %w", err)
	}

	if err := r.database.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	return nil
}

func (r *PatchingRepository) GetApplicationVersion(ctx context.Context, name string) (patch.Version, error) {
	result, err := r.queries.GetApplicationVersion(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return patch.Version{}, nil
		}
		return patch.Version{}, err
	}
	return patch.Version{
		Major: int(result.Major),
		Minor: int(result.Minor),
		Patch: int(result.Patch),
	}, nil
}

func (r *PatchingRepository) UpdateApplicationVersion(ctx context.Context, name string, version patch.Version) error {
	var args = mevmanifest.CreateApplicationVersionParams{
		Major: int64(version.Major),
		Minor: int64(version.Minor),
		Patch: int64(version.Patch),
		Name:  name,
	}
	return r.queries.CreateApplicationVersion(ctx, args)
}

func (r *PatchingRepository) GetApplicationFile(ctx context.Context, application string, path string) (file.LocalFile, error) {
	result, err := r.queries.GetApplicationFile(ctx, mevmanifest.GetApplicationFileParams{
		Path:        path,
		Application: application,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return file.LocalFile{}, nil
		}
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
