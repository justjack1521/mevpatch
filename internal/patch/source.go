package patch

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/file"
	uuid "github.com/satori/go.uuid"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type RemoteFileSourceJob struct {
	JobID uuid.UUID
	Path  string
}

type RemoteFileSourceWorker struct {
	wg          *sync.WaitGroup
	application string
	host        string
	sources     <-chan *RemoteFileSourceJob
	commit      chan<- *FileMetadataCommitJob
	progress    chan<- float32
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

func (g *RemoteFileSourceWorkerGroup) Start(app string, host string, output chan<- *FileMetadataCommitJob, progress chan<- float32, errors chan<- error) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		var worker = NewRemoteFileSourceWorker(g.wg, app, host, g.channel, output, progress, errors)
		go worker.Run()
	}
}

func NewRemoteFileSourceWorker(wg *sync.WaitGroup, app string, host string, input <-chan *RemoteFileSourceJob, output chan<- *FileMetadataCommitJob, progress chan<- float32, errors chan<- error) *RemoteFileSourceWorker {
	return &RemoteFileSourceWorker{wg: wg, application: app, host: host, sources: input, commit: output, progress: progress, errors: errors}
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

	destination, err := file.PersistentPath(w.application, job.Path)
	if err != nil {
		return fmt.Errorf("failed to get persistent file: %w", err)
	}

	uri, err := url.JoinPath(w.host, "downloads", w.application, "source", job.Path)
	if err != nil {
		return fmt.Errorf("failed to get join url path: %w", err)
	}

	response, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("failed to get remote file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d for %s", response.StatusCode, uri)
	}

	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination path: %w", err)
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer out.Close()

	var total int64

	var buffer = make([]byte, 1024*16)
	for {
		next, err := response.Body.Read(buffer)
		if next > 0 {
			written, err := out.Write(buffer[:next])
			if err != nil {
				return fmt.Errorf("failed to write to buffer: %w", err)
			}
			total += int64(written)
			w.progress <- float32(next)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read from buffer: %s", err)
		}
	}

	checksum, err := GetChecksumForPath(destination)
	if err != nil {
		return fmt.Errorf("failed to get checksum for path: %w", err)
	}

	w.commit <- &FileMetadataCommitJob{
		Path:     job.Path,
		Size:     total,
		Checksum: checksum,
	}

	return nil

}
