package update

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"sync"
)

type FileCategorisationReason int

const (
	FileCategorisationReasonNone = iota
	FileCategorisationReasonNotFoundDatabase
	FileCategorisationReasonNotFoundDisk
	FileCategorisationReasonDatabaseMismatch
	FileCategorisationReasonForceFull
)

type FileResultCategory int

type FileCategorisationResult struct {
	Category FileResultCategory
	Reason   FileCategorisationReason
	File     *mevmanifest.File
}

var (
	FileCategorisationReasonPrintMap = map[FileCategorisationReason]string{
		FileCategorisationReasonNone:             "None",
		FileCategorisationReasonNotFoundDatabase: "Not found in database",
		FileCategorisationReasonNotFoundDisk:     "Not found on disk",
		FileCategorisationReasonDatabaseMismatch: "Database mismatch",
		FileCategorisationReasonForceFull:        "Force",
	}
)

const (
	FileResultIgnore FileResultCategory = iota
	FileResultPatch
	FileResultDownload
)

type PlanningResultCollector struct {
	FilesRequireDownload []*mevmanifest.File
	FilesRequirePatch    []*mevmanifest.File
	FilesIgnored         []*mevmanifest.File

	channel chan *FileCategorisationResult
	done    chan struct{}
	mu      sync.Mutex
}

func (c *PlanningResultCollector) Total() int {
	return len(c.FilesIgnored) + len(c.FilesRequirePatch) + len(c.FilesRequireDownload)
}

func (c *PlanningResultCollector) TotalCategory(category FileResultCategory) int {
	switch category {
	case FileResultIgnore:
		return len(c.FilesIgnored)
	case FileResultPatch:
		return len(c.FilesRequirePatch)
	case FileResultDownload:
		return len(c.FilesRequireDownload)
	}
	return 0
}

func NewPlanningResultCollector() *PlanningResultCollector {
	return &PlanningResultCollector{
		FilesRequireDownload: make([]*mevmanifest.File, 0),
		FilesRequirePatch:    make([]*mevmanifest.File, 0),
		FilesIgnored:         make([]*mevmanifest.File, 0),
		channel:              make(chan *FileCategorisationResult),
		done:                 make(chan struct{}),
	}
}

func (c *PlanningResultCollector) Start() {
	for result := range c.channel {
		c.mu.Lock()
		switch result.Category {
		case FileResultIgnore:
			fmt.Println(fmt.Sprintf("[Result Collector] Ignore %s %s", FileCategorisationReasonPrintMap[result.Reason], result.File.Path))
			c.FilesIgnored = append(c.FilesIgnored, result.File)
		case FileResultPatch:
			fmt.Println(fmt.Sprintf("[Result Collector] Patch %s %s", FileCategorisationReasonPrintMap[result.Reason], result.File.Path))
			c.FilesRequirePatch = append(c.FilesRequirePatch, result.File)
		case FileResultDownload:
			fmt.Println(fmt.Sprintf("[Result Collector] Download %s %s", FileCategorisationReasonPrintMap[result.Reason], result.File.Path))
			c.FilesRequireDownload = append(c.FilesRequireDownload, result.File)
		}
		c.mu.Unlock()
	}
	close(c.done)
}
