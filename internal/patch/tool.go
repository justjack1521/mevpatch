package patch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// MergeTool wraps hpatchz, which applies a binary diff to a target file in-place.
// Command: hpatchz -f <target> <patch> <target>
type MergeTool struct {
	Path string // full path to the hpatchz executable
	Dir  string // temp directory to remove on cleanup
}

func NewMergeTool(path string) *MergeTool {
	return &MergeTool{Path: path}
}

// Apply applies patchFile to targetFile, overwriting it in-place.
func (m *MergeTool) Apply(targetFile string, patchFile string) error {
	cmd := newCmd(m.Path, "-f", targetFile, patchFile, targetFile)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hpatchz failed on %q: %w (stderr: %s)", targetFile, err, stderr.String())
	}
	return nil
}

// CreateMergeTool extracts the embedded hpatchz binary to a temp directory.
// Caller must defer os.RemoveAll(tool.Dir) to clean up.
func CreateMergeTool(binary []byte) (*MergeTool, error) {
	dir, err := os.MkdirTemp("", "hpatchz-*")
	if err != nil {
		return nil, fmt.Errorf("creating hpatchz temp dir: %w", err)
	}
	path := filepath.Join(dir, "hpatchz.exe")
	if err := os.WriteFile(path, binary, 0755); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("writing hpatchz binary: %w", err)
	}
	return &MergeTool{Path: path, Dir: dir}, nil
}
