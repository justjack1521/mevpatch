package update

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type LocalFileValidateJob struct {
	Path         string
	Size         int64
	LastModified time.Time
}

type LocalFileValidateWorker struct {
	wg        *sync.WaitGroup
	validates <-chan *LocalFileValidateJob
	sourcers  chan<- *RemoteFileSourceJob
	errors    chan<- error
}

type LocalFileValidateWorkerGroup struct {
	wg      *sync.WaitGroup
	channel chan *LocalFileValidateJob
	count   int
}

func NewLocalFileValidateWorkerGroup(count int) *LocalFileValidateWorkerGroup {
	return &LocalFileValidateWorkerGroup{
		wg:      &sync.WaitGroup{},
		channel: make(chan *LocalFileValidateJob, count),
		count:   count,
	}
}

func (g *LocalFileValidateWorkerGroup) Start(sourcers chan<- *RemoteFileSourceJob, errors chan<- error) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewValidateJobWorker(g.wg, g.channel, sourcers, errors)
		go worker.Run()
	}
}

func (g *LocalFileValidateWorkerGroup) Wait() {
	g.wg.Wait()
}

func NewValidateJobWorker(wg *sync.WaitGroup, input <-chan *LocalFileValidateJob, output chan<- *RemoteFileSourceJob, errors chan<- error) *LocalFileValidateWorker {
	return &LocalFileValidateWorker{wg: wg, validates: input, sourcers: output, errors: errors}
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

	destination, err := PersistentPath(job.Path)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("Validating file at persistent path %s", destination))

	stat, err := os.Stat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			w.sourcers <- &RemoteFileSourceJob{
				Path:        job.Path,
				Destination: destination,
			}
			return nil
		}
		return ErrLocalFileValidationFailed(job, err)
	}

	if stat.Size() != job.Size || !stat.ModTime().UTC().Equal(job.LastModified) == false {
		w.sourcers <- &RemoteFileSourceJob{
			Path:        job.Path,
			Destination: destination,
		}
	}

	return nil

}
