package patch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type Merger struct {
	Path string
}

func NewMerger(path string) *Merger {
	return &Merger{Path: path}
}

func (m *Merger) Merge(patch string, target string) error {
	var cmd = exec.Command(m.Path, patch, target)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w; stderr: %s", err, stderr.String())
	}
	return nil
}

func CreateMerger(bytes []byte) (*Merger, error) {

	file, err := os.CreateTemp("", "jpatch-*.exe")
	if err != nil {
		return nil, err
	}

	if _, err := file.Write(bytes); err != nil {
		return nil, err
	}

	if err := file.Close(); err != nil {
		return nil, err
	}

	return NewMerger(file.Name()), nil

}
