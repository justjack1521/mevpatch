package orchestrate

import (
	"context"
	"github.com/justjack1521/mevpatch/internal/database"
	"github.com/justjack1521/mevpatch/internal/patch"
)

const StatusUpdateConnectingDatabase = "Connecting to file cache"
const StatusUpdateDownloadDatabase = "File cache not found, downloading"
const StatusUpdateScanDatabase = "Scanning files into database"

type DatabaseInitialiseStep struct {
	orchestrator *Orchestrator
}

func NewDatabaseInitialiseStep(orchestrator *Orchestrator) *DatabaseInitialiseStep {
	return &DatabaseInitialiseStep{orchestrator: orchestrator}
}

func (s *DatabaseInitialiseStep) Run(ctx context.Context) (*database.PatchingRepository, error) {

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdateConnectingDatabase)

	var dbe = database.ExistsAtPath(s.orchestrator.Application)

	if dbe == false {

		s.orchestrator.SendSecondaryStatusUpdate(StatusUpdateDownloadDatabase)

		var minor = patch.Version{
			Major: s.orchestrator.TargetVersion.Major,
			Minor: s.orchestrator.TargetVersion.Minor,
			Patch: 0,
		}

		if err := database.DownloadDatabase("https://mevius-patch-us.sfo3.digitaloceanspaces.com", s.orchestrator.Application, minor); err != nil {
			return nil, err
		}

		s.orchestrator.SendSecondaryStatusUpdate(StatusUpdateScanDatabase)

	}

	dbc, err := database.NewConnection(s.orchestrator.Application)
	if err != nil {
		return nil, err
	}

	return database.NewPatchingRepository(dbc), nil

}
