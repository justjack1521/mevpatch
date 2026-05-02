package orchestrate

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	"sync"
)

// VerifyResult is the outcome for a single file.
type VerifyResult struct {
	Path     string
	Status   VerifyStatus
	Expected string // manifest checksum
	Actual   string // checksum on disk (empty if missing)
}

type VerifyStatus int

const (
	VerifyOK      VerifyStatus = iota
	VerifyMissing              // file not on disk
	VerifyCorrupt              // checksum mismatch
)

// VerifyReport is the full output of a verify run.
type VerifyReport struct {
	Version string
	Total   int
	OK      int
	Missing []VerifyResult
	Corrupt []VerifyResult
}

func (r *VerifyReport) Clean() bool {
	return len(r.Missing) == 0 && len(r.Corrupt) == 0
}

// VerifyStep downloads the manifest for the currently installed version and
// checksums every file on disk against it. It does not modify any files.
type VerifyStep struct {
	Report *VerifyReport
}

func NewVerifyStep() *VerifyStep {
	return &VerifyStep{Report: &VerifyReport{}}
}

func (s *VerifyStep) Run(ctx *Context, o *Orchestrator) error {
	if ctx.CurrentVersion.Zero() {
		return fmt.Errorf("no installed version found — run patch first")
	}

	s.Report.Version = ctx.CurrentVersion.String()
	o.SendPrimaryStatusUpdate(fmt.Sprintf("Verifying %s %s...", ctx.ApplicationName, ctx.CurrentVersion.String()))
	o.SendSecondaryStatusUpdate("Downloading manifest...")

	mani, err := downloadManifest(ctx.ApplicationName, ctx.CurrentVersion)
	if err != nil {
		return fmt.Errorf("downloading manifest for %s: %w", ctx.CurrentVersion.String(), err)
	}

	s.Report.Total = len(mani.Files)
	o.SendSecondaryStatusUpdate(fmt.Sprintf("Checking %d files...", s.Report.Total))
	o.ResetSecondaryProgress()

	results := make(chan VerifyResult, len(mani.Files))
	s.checkFiles(ctx.ApplicationName, mani.Files, results, float32(len(mani.Files)), o)

	for r := range results {
		switch r.Status {
		case VerifyOK:
			s.Report.OK++
		case VerifyMissing:
			s.Report.Missing = append(s.Report.Missing, r)
		case VerifyCorrupt:
			s.Report.Corrupt = append(s.Report.Corrupt, r)
		}
	}

	if s.Report.Clean() {
		o.SendPrimaryStatusUpdate("All files verified successfully")
		o.SendSecondaryStatusUpdate(fmt.Sprintf("%d/%d files OK", s.Report.OK, s.Report.Total))
	} else {
		o.SendPrimaryStatusUpdate("Verification failed")
		o.SendSecondaryStatusUpdate(fmt.Sprintf(
			"%d OK  •  %d missing  •  %d corrupt",
			s.Report.OK, len(s.Report.Missing), len(s.Report.Corrupt),
		))
		for _, f := range s.Report.Missing {
			o.SendLog("MISSING: %s", f.Path)
		}
		for _, f := range s.Report.Corrupt {
			o.SendLog("CORRUPT: %s", f.Path)
		}
	}

	return nil
}

func (s *VerifyStep) checkFiles(
	app string,
	files []*mevmanifest.File,
	out chan<- VerifyResult,
	total float32,
	o *Orchestrator,
) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		checked float32
	)

	sem := make(chan struct{}, 8) // 8 concurrent checksummers

	for _, f := range files {
		wg.Add(1)
		f := f
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := checkFile(app, f)
			out <- result

			mu.Lock()
			checked++
			o.SendSecondaryProgressUpdate(checked / total)
			mu.Unlock()
		}()
	}

	wg.Wait()
	close(out)
}

func checkFile(app string, f *mevmanifest.File) VerifyResult {
	path, err := file.PersistentPath(app, f.Path)
	if err != nil {
		return VerifyResult{Path: f.Path, Status: VerifyMissing, Expected: f.Checksum}
	}

	actual, err := patch.GetChecksumForPath(path)
	if err != nil {
		// os.IsNotExist or unreadable — treat as missing
		return VerifyResult{Path: f.Path, Status: VerifyMissing, Expected: f.Checksum}
	}

	if actual != f.Checksum {
		return VerifyResult{Path: f.Path, Status: VerifyCorrupt, Expected: f.Checksum, Actual: actual}
	}

	return VerifyResult{Path: f.Path, Status: VerifyOK, Expected: f.Checksum, Actual: actual}
}
