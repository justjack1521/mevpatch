package patch

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/justjack1521/mevpatch/internal/file"
)

// FileMetadataCommitJob carries updated metadata after a successful patch or download.
type FileMetadataCommitJob struct {
	Path     string
	Size     int64
	Checksum string
}

func NewFileMetadataCommitJob(path string, size int64, checksum string) *FileMetadataCommitJob {
	return &FileMetadataCommitJob{Path: path, Size: size, Checksum: checksum}
}

// InstallState is the on-disk JSON record of what the patcher last verified.
// It lives at game_root/Patching/{app}.version
type InstallState struct {
	Version   string           `json:"Version"`
	UpdatedAt time.Time        `json:"UpdatedAt"`
	Files     []file.LocalFile `json:"Files"`
}

func (s *InstallState) Upsert(path string, size int64, checksum string) {
	for i, f := range s.Files {
		if f.Path == path {
			s.Files[i].Size = size
			s.Files[i].Checksum = checksum
			s.Files[i].Timestamp = time.Now().UTC()
			return
		}
	}
	s.Files = append(s.Files, file.LocalFile{
		Path:      path,
		Size:      size,
		Checksum:  checksum,
		Timestamp: time.Now().UTC(),
	})
}

func (s *InstallState) Find(path string) (file.LocalFile, bool) {
	for _, f := range s.Files {
		if f.Path == path {
			return f, true
		}
	}
	return file.LocalFile{}, false
}

// LoadInstallState reads the state file for an app. Returns an empty state if not found.
func LoadInstallState(app string) (*InstallState, error) {
	path, err := file.VersionFilePath(app)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &InstallState{Files: []file.LocalFile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading install state: %w", err)
	}
	var state InstallState
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupt state file — start fresh rather than failing.
		fmt.Printf("[State] Warning: corrupt state file for %s, starting fresh\n", app)
		return &InstallState{Files: []file.LocalFile{}}, nil
	}
	return &state, nil
}

// SaveInstallState atomically writes the state file.
func SaveInstallState(app string, state *InstallState) error {
	path, err := file.VersionFilePath(app)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling install state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing install state: %w", err)
	}
	return os.Rename(tmp, path)
}

// FileMetadataCommitWorker persists commit jobs back to an InstallState.
type FileMetadataCommitWorker struct {
	app     string
	state   *InstallState
	mu      *sync.Mutex
	group   *sync.WaitGroup
	commits <-chan *FileMetadataCommitJob
	errors  chan<- error
}

func NewFileMetadataCommitWorker(
	app string,
	state *InstallState,
	mu *sync.Mutex,
	group *sync.WaitGroup,
	input <-chan *FileMetadataCommitJob,
	errors chan<- error,
) *FileMetadataCommitWorker {
	return &FileMetadataCommitWorker{
		app: app, state: state, mu: mu,
		group: group, commits: input, errors: errors,
	}
}

func (w *FileMetadataCommitWorker) Run() {
	defer w.group.Done()
	for job := range w.commits {
		w.mu.Lock()
		w.state.Upsert(job.Path, job.Size, job.Checksum)
		w.mu.Unlock()
	}
}

// FileMetadataCommitWorkerGroup manages a pool of commit workers sharing a state.
type FileMetadataCommitWorkerGroup struct {
	state   *InstallState
	mu      sync.Mutex
	group   *sync.WaitGroup
	Channel chan *FileMetadataCommitJob
	count   int
}

func NewFileMetadataCommitWorkerGroup(state *InstallState, count int) *FileMetadataCommitWorkerGroup {
	return &FileMetadataCommitWorkerGroup{
		state:   state,
		group:   &sync.WaitGroup{},
		Channel: make(chan *FileMetadataCommitJob, count),
		count:   count,
	}
}

func (g *FileMetadataCommitWorkerGroup) Start(app string, errors chan<- error) {
	g.group.Add(g.count)
	for i := 0; i < g.count; i++ {
		w := NewFileMetadataCommitWorker(app, g.state, &g.mu, g.group, g.Channel, errors)
		go w.Run()
	}
}

func (g *FileMetadataCommitWorkerGroup) Wait() {
	g.group.Wait()
}
