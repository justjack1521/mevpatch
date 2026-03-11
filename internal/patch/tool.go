package patch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type MergeTool struct {
	Path string
}

func NewMergeTool(path string) *MergeTool {
	return &MergeTool{Path: path}
}

func (m *MergeTool) Merge(target string, patch string) error {

	var cmd = exec.Command(m.Path, "-f", target, patch, target)

	fmt.Println(cmd.String())

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w; stderr: %s", err, stderr.String())
	}

	return nil
}

func CreateMergeTool(bytes []byte) (*MergeTool, error) {

	tempDir, err := os.MkdirTemp("", "hpatchz-*")
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(tempDir, "hpatchz.exe")

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	if _, err := file.Write(bytes); err != nil {
		os.Remove(file.Name())
		return nil, err
	}

	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return nil, err
	}

	if err := os.Chmod(filePath, 0755); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, err
	}

	return NewMergeTool(file.Name()), nil

}
