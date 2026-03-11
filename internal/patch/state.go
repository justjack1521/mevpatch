package patch

import (
	"context"
	"github.com/justjack1521/mevpatch/internal/file"
	uuid "github.com/satori/go.uuid"
	"time"
)

type State struct {
	ID         uuid.UUID        `json:"ID"`
	RemoteHash string           `json:"RemoteHash"`
	LocalHash  string           `json:"LocalHash"`
	VerifiedAt time.Time        `json:"VerifiedAt"`
	Files      []file.LocalFile `json:"Files"`
}

func (x *State) GetApplicationFile(ctx context.Context, application string, path string) (file.LocalFile, error) {
	for _, value := range x.Files {
		if value.Path == path {
			return value, nil
		}
	}
	return file.LocalFile{}, nil
}

func (x *State) GetApplicationFiles(ctx context.Context, name string) ([]file.LocalFile, error) {
	return x.Files, nil
}

func (x *State) CreateApplicationFile(ctx context.Context, app string, path string, size int64, check string, t time.Time) error {
	x.Files = append(x.Files, file.LocalFile{
		Path:      path,
		Size:      size,
		Checksum:  check,
		Timestamp: t,
	})
	return nil
}
