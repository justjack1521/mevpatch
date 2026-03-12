package update

import (
	"fmt"
	"sync"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
)

type FileResultCategory int

const (
	FileResultIgnore   FileResultCategory = iota
	FileResultPatch                       // has a patch bundle entry from current version
	FileResultDownload                    // needs full source download
)

type FileCategorisationReason int

const (
	ReasonUpToDate      FileCategorisationReason = iota
	ReasonNotOnDisk                              // file missing entirely
	ReasonChecksumDiff                           // file present but checksum differs
	ReasonNoPatchBundle                          // no bundle available for current version
	ReasonRebase                                 // current version outside patch window
)

var reasonLabel = map[FileCategorisationReason]string{
	ReasonUpToDate:      "up-to-date",
	ReasonNotOnDisk:     "not on disk",
	ReasonChecksumDiff:  "checksum mismatch",
	ReasonNoPatchBundle: "no patch bundle",
	ReasonRebase:        "rebase required",
}

type FileCategorisationResult struct {
	Category FileResultCategory
	Reason   FileCategorisationReason
	File     *mevmanifest.File
}

// PlanningResultCollector receives categorisation results from workers and
// sorts them into the three buckets used by later pipeline stages.
type PlanningResultCollector struct {
	FilesRequireDownload []*mevmanifest.File
	FilesRequirePatch    []*mevmanifest.File
	FilesIgnored         []*mevmanifest.File

	channel chan *FileCategorisationResult
	done    chan struct{}
	mu      sync.Mutex
}

func NewPlanningResultCollector() *PlanningResultCollector {
	return &PlanningResultCollector{
		FilesRequireDownload: make([]*mevmanifest.File, 0),
		FilesRequirePatch:    make([]*mevmanifest.File, 0),
		FilesIgnored:         make([]*mevmanifest.File, 0),
		channel:              make(chan *FileCategorisationResult, 64),
		done:                 make(chan struct{}),
	}
}

func (c *PlanningResultCollector) Start() {
	for result := range c.channel {
		c.mu.Lock()
		label := reasonLabel[result.Reason]
		switch result.Category {
		case FileResultIgnore:
			fmt.Printf("[Plan] ok      %s (%s)\n", result.File.Path, label)
			c.FilesIgnored = append(c.FilesIgnored, result.File)
		case FileResultPatch:
			fmt.Printf("[Plan] patch   %s (%s)\n", result.File.Path, label)
			c.FilesRequirePatch = append(c.FilesRequirePatch, result.File)
		case FileResultDownload:
			fmt.Printf("[Plan] download %s (%s)\n", result.File.Path, label)
			c.FilesRequireDownload = append(c.FilesRequireDownload, result.File)
		}
		c.mu.Unlock()
	}
	close(c.done)
}

func (c *PlanningResultCollector) Total() int {
	return len(c.FilesIgnored) + len(c.FilesRequirePatch) + len(c.FilesRequireDownload)
}

func (c *PlanningResultCollector) TotalCategory(cat FileResultCategory) int {
	switch cat {
	case FileResultIgnore:
		return len(c.FilesIgnored)
	case FileResultPatch:
		return len(c.FilesRequirePatch)
	case FileResultDownload:
		return len(c.FilesRequireDownload)
	}
	return 0
}
