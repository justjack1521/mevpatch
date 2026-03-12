package patch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/justjack1521/mevpatch/internal/file"
)

// RemoteFileSourceJob is a request to download a single source file.
type RemoteFileSourceJob struct {
	Path     string
	Checksum string
	Size     int64
}

type RemoteFileSourceWorker struct {
	wg          *sync.WaitGroup
	application string
	host        string
	sources     <-chan *RemoteFileSourceJob
	commits     chan<- *FileMetadataCommitJob
	progress    chan<- float32
	errors      chan<- error
}

func NewRemoteFileSourceWorker(
	wg *sync.WaitGroup,
	app, host string,
	input <-chan *RemoteFileSourceJob,
	commits chan<- *FileMetadataCommitJob,
	progress chan<- float32,
	errors chan<- error,
) *RemoteFileSourceWorker {
	return &RemoteFileSourceWorker{
		wg: wg, application: app, host: host,
		sources: input, commits: commits, progress: progress, errors: errors,
	}
}

func (w *RemoteFileSourceWorker) Run() {
	defer w.wg.Done()
	for job := range w.sources {
		fmt.Printf("[Source] Downloading: %s\n", job.Path)
		if err := w.run(job); err != nil {
			fmt.Printf("[Source] Failed: %s: %v\n", job.Path, err)
			w.errors <- err
		} else {
			fmt.Printf("[Source] Done: %s\n", job.Path)
		}
	}
}

func (w *RemoteFileSourceWorker) run(job *RemoteFileSourceJob) error {
	destination, err := file.PersistentPath(w.application, job.Path)
	if err != nil {
		return fmt.Errorf("resolving path for %s: %w", job.Path, err)
	}

	uri, err := url.JoinPath(w.host, "downloads", w.application, "source", job.Path)
	if err != nil {
		return fmt.Errorf("building URL for %s: %w", job.Path, err)
	}

	resp, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("GET %s: %w", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, uri)
	}

	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return fmt.Errorf("creating directories for %s: %w", destination, err)
	}

	// Write to a temp file first so a failed download doesn't leave a corrupt file.
	tmp := destination + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", destination, err)
	}

	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			wn, writeErr := out.Write(buf[:n])
			written += int64(wn)
			w.progress <- float32(wn)
			if writeErr != nil {
				out.Close()
				os.Remove(tmp)
				return fmt.Errorf("writing %s: %w", destination, writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			os.Remove(tmp)
			return fmt.Errorf("reading response for %s: %w", job.Path, readErr)
		}
	}
	out.Close()

	// Verify checksum before committing.
	checksum, err := GetChecksumForPath(tmp)
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("checksumming download for %s: %w", job.Path, err)
	}
	if checksum != job.Checksum {
		os.Remove(tmp)
		return fmt.Errorf("checksum mismatch for %s: expected %s got %s", job.Path, job.Checksum, checksum)
	}

	if err := os.Rename(tmp, destination); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("committing download for %s: %w", job.Path, err)
	}

	w.commits <- &FileMetadataCommitJob{Path: job.Path, Size: written, Checksum: checksum}
	return nil
}

// RemoteFileSourceWorkerGroup manages a pool of source download workers.
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

func (g *RemoteFileSourceWorkerGroup) Start(
	app, host string,
	commits chan<- *FileMetadataCommitJob,
	progress chan<- float32,
	errors chan<- error,
) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		w := NewRemoteFileSourceWorker(g.wg, app, host, g.channel, commits, progress, errors)
		go w.Run()
	}
}

func (g *RemoteFileSourceWorkerGroup) Wait() { g.wg.Wait() }
