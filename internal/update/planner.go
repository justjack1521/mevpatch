package update

import (
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/database"
)

type Planner struct {
	application string
	repository  *database.PatchingRepository
	collector   *PlanningResultCollector
	validators  *RemoteFileValidateWorkerGroup
	errors      chan error
}

func NewPlanner(app string, repo *database.PatchingRepository) *Planner {
	return &Planner{
		application: app,
		repository:  repo,
		collector:   NewPlanningResultCollector(),
		validators:  NewRemoteFileValidateWorkerGroup(app, repo, 1),
		errors:      make(chan error, 10),
	}
}

func (p *Planner) Start(files []*mevmanifest.File) {

	go func() {
		for err := range p.errors {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	go p.collector.Start()
	p.validators.Start(p.collector.channel, p.errors)

	for _, file := range files {
		p.validators.channel <- &RemoteFileValidateJob{
			file: file,
		}
	}

	close(p.validators.channel)
	p.validators.Wait()
	close(p.collector.channel)
	close(p.errors)

	fmt.Println(fmt.Sprintf("[Total Files Collected] %d", p.collector.Total()))
	fmt.Println(fmt.Sprintf("[Total Files Ignored] %d", p.collector.TotalCategory(FileResultIgnore)))
	fmt.Println(fmt.Sprintf("[Total Files Patching] %d", p.collector.TotalCategory(FileResultPatch)))
	fmt.Println(fmt.Sprintf("[Total Files Downloading] %d", p.collector.TotalCategory(FileResultDownload)))

}
