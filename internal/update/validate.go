package update

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	uuid "github.com/satori/go.uuid"
	"os"
	"sync"
	"time"
)

type LocalFileValidateJob struct {
	JobID        uuid.UUID
	Path         string
	Size         int64
	LastModified time.Time
}

type LocalFileValidateWorker struct {
	app       string
	wg        *sync.WaitGroup
	validates <-chan *LocalFileValidateJob
	sourcers  chan<- *patch.RemoteFileSourceJob
	errors    chan<- error
}

type LocalFileValidateWorkerGroup struct {
	app     string
	wg      *sync.WaitGroup
	channel chan *LocalFileValidateJob
	count   int
}

func NewLocalFileValidateWorkerGroup(app string, count int) *LocalFileValidateWorkerGroup {
	return &LocalFileValidateWorkerGroup{
		app:     app,
		wg:      &sync.WaitGroup{},
		channel: make(chan *LocalFileValidateJob, count),
		count:   count,
	}
}

func (g *LocalFileValidateWorkerGroup) Start(sourcers chan<- *patch.RemoteFileSourceJob, errors chan<- error) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewValidateJobWorker(g.app, g.wg, g.channel, sourcers, errors)
		go worker.Run()
	}
}

func (g *LocalFileValidateWorkerGroup) Wait() {
	g.wg.Wait()
}

func NewValidateJobWorker(app string, wg *sync.WaitGroup, input <-chan *LocalFileValidateJob, output chan<- *patch.RemoteFileSourceJob, errors chan<- error) *LocalFileValidateWorker {
	return &LocalFileValidateWorker{app: app, wg: wg, validates: input, sourcers: output, errors: errors}
}

func (w *LocalFileValidateWorker) Run() {

	defer w.wg.Done()

	for job := range w.validates {
		if err := w.run(job); err != nil {
			w.errors <- err
			continue
		}
	}

}

var (
	ErrLocalFileValidationFailed = func(job *LocalFileValidateJob, err error) error {
		return fmt.Errorf("local file validation failed for %s: %w", job.Path, err)
	}
)

func (w *LocalFileValidateWorker) run(job *LocalFileValidateJob) error {

	fmt.Println(fmt.Sprintf("Validating %s", job.Path))

	destination, err := file.PersistentPath(w.app, job.Path)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("Validating file at persistent path %s", destination))

	stat, err := os.Stat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			w.sourcers <- &patch.RemoteFileSourceJob{
				JobID: job.JobID,
				Path:  job.Path,
			}
			return nil
		}
		return ErrLocalFileValidationFailed(job, err)
	}

	if stat.Size() != job.Size || !stat.ModTime().UTC().Equal(job.LastModified) == false {
		w.sourcers <- &patch.RemoteFileSourceJob{
			Path: job.Path,
		}
	}

	return nil

}
