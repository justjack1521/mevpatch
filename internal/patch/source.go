package patch

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/justjack1521/mevpatch/internal/file"
)

var httpClient = &http.Client{}

var http1Client = &http.Client{
	Transport: &http.Transport{
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	},
}

// ForceHTTP1Client forces HTTP/1.1 for all downloads.
func ForceHTTP1Client() {
	httpClient = http1Client
}

func isStreamError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "stream error") ||
		strings.Contains(s, "INTERNAL_ERROR") ||
		strings.Contains(s, "CANCEL") ||
		strings.Contains(s, "PROTOCOL_ERROR")
}

// RemoteFileSourceJob is a request to download a single source file.
type RemoteFileSourceJob struct {
	Path          string
	Checksum      string
	Size          int64
	SourceVersion string // version folder to download from e.g. "1.0.0"
}

// SourceProgress is sent on the progress channel during a source download.
type SourceProgress struct {
	BytesRead int
	FileDone  bool
}

type RemoteFileSourceWorker struct {
	wg          *sync.WaitGroup
	application string
	host        string
	sources     <-chan *RemoteFileSourceJob
	commits     chan<- *FileMetadataCommitJob
	progress    chan<- SourceProgress
	errors      chan<- error
}

func NewRemoteFileSourceWorker(
	wg *sync.WaitGroup,
	app, host string,
	input <-chan *RemoteFileSourceJob,
	commits chan<- *FileMetadataCommitJob,
	progress chan<- SourceProgress,
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
		if err := w.run(job); err != nil {
			w.errors <- err
		}
	}
}

func (w *RemoteFileSourceWorker) run(job *RemoteFileSourceJob) error {
	err := w.attempt(job, httpClient)
	if err != nil && isStreamError(err) {
		err = w.attempt(job, http1Client)
	}
	return err
}

func (w *RemoteFileSourceWorker) attempt(job *RemoteFileSourceJob, client *http.Client) error {
	destination, err := file.PersistentPath(w.application, job.Path)
	if err != nil {
		return fmt.Errorf("resolving path for %s: %w", job.Path, err)
	}

	u := &url.URL{
		Scheme: "https",
		Host:   strings.TrimPrefix(strings.TrimPrefix(w.host, "https://"), "http://"),
		Path:   "/downloads/" + w.application + "/src/" + job.SourceVersion + "/" + job.Path,
	}
	uri := u.String()

	resp, err := client.Get(uri)
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
			w.progress <- SourceProgress{BytesRead: wn}
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

	w.progress <- SourceProgress{FileDone: true}
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
	progress chan<- SourceProgress,
	errors chan<- error,
) {
	g.wg.Add(g.count)
	for i := 0; i < g.count; i++ {
		w := NewRemoteFileSourceWorker(g.wg, app, host, g.channel, commits, progress, errors)
		go w.Run()
	}
}

func (g *RemoteFileSourceWorkerGroup) Wait() { g.wg.Wait() }
