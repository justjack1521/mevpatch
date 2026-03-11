package orchestrate

import (
	"context"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
)

const (
	StatusUpdateApplyPatch = "Applying patches"
)

type PatchMergeStep struct {
	orchestrator *Orchestrator
	merger       *patch.MergeTool
	repository   file.Repository

	ProgressCount float32
	ProgressTotal float32
}

func NewPatchMergeStep(orchestrator *Orchestrator, repository file.Repository) *PatchMergeStep {
	return &PatchMergeStep{orchestrator: orchestrator, repository: repository}
}

func (s *PatchMergeStep) Run(ctx context.Context, jobs []*patch.RemoteFileMergeJob) error {

	s.orchestrator.SendPrimaryStatusUpdate(StatusUpdateApplyPatch)
	s.orchestrator.ResetSecondaryProgress()

	s.ProgressCount = 0
	s.ProgressTotal = float32(len(jobs))

	var merger = patch.NewRemotePatchFileMerger(s.orchestrator.Application, s.orchestrator.Merger, s.repository)
	merger.Start(jobs)

	return nil

}
