package update

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type RemoteFileSourceJob struct {
	Path        string
	Destination string
}

type RemoteFileSourceWorker struct {
	wg          *sync.WaitGroup
	application string
	host        string
	sources     <-chan *RemoteFileSourceJob
	commit      chan<- *FileMetadataCommitJob
	errors      chan<- error
}

type RemoteFileSourceWorkerGroup struct {
	wg      *sync.WaitGroup
	channel chan *RemoteFileSourceJob
	count   int
}

func NewRemoteFileSourceWorkerGroup(count int) *RemoteFileSourceWorkerGroup {
	return &RemoteFileSourceWorkerGroup{
		wg:      &sync.WaitGroup{},
		channel: make(chan *RemoteFileSourceJob, count),
		count:   count,
	}
}

func (g *RemoteFileSourceWorkerGroup) Wait() {
	g.wg.Wait()
}

func (g *RemoteFileSourceWorkerGroup) Start(app string, host string, output chan<- *FileMetadataCommitJob, errors chan<- error) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewRemoteFileSourceWorker(g.wg, app, host, g.channel, output, errors)
		go worker.Run()
	}
}

func NewRemoteFileSourceWorker(wg *sync.WaitGroup, app string, host string, input <-chan *RemoteFileSourceJob, output chan<- *FileMetadataCommitJob, errors chan<- error) *RemoteFileSourceWorker {
	return &RemoteFileSourceWorker{wg: wg, application: app, host: host, sources: input, commit: output, errors: errors}
}

func (w *RemoteFileSourceWorker) Run() {

	defer w.wg.Done()

	for job := range w.sources {
		fmt.Println(fmt.Sprintf("[Remote Sourcing] Started: %s", job.Path))
		if err := w.run(job); err != nil {
			fmt.Println(fmt.Sprintf("[Remote Sourcing] Failed: %s", job.Path))
			w.errors <- err
			continue
		}
		fmt.Println(fmt.Sprintf("[Remote Sourcing] Completed: %s", job.Path))
	}

}

func (w *RemoteFileSourceWorker) run(job *RemoteFileSourceJob) error {

	uri, err := url.JoinPath(w.host, "downloads", w.application, "source", job.Path)
	if err != nil {
		return err
	}

	response, err := http.Get(uri)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d for %s", response.StatusCode, uri)
	}

	if err := os.MkdirAll(filepath.Dir(job.Destination), os.ModePerm); err != nil {
		return err
	}

	out, err := os.Create(job.Destination)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, response.Body)
	if err != nil {
		return err
	}

	w.commit <- &FileMetadataCommitJob{
		Path: job.Path,
		Size: written,
	}

	return nil

}
