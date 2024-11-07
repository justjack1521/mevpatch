package update

import (
	"os"
	"path/filepath"
)

func TemporaryPath(app string, normal string) (string, error) {
	var path = filepath.Join(os.TempDir(), "mevpatch", app, normal)
	return filepath.Clean(path), nil
}

func PersistentPath(normal string) (string, error) {
	own, err := os.Executable()
	if err != nil {
		return "", err
	}

	var path = filepath.Join(filepath.Dir(own), "..", "..", normal)
	return filepath.Clean(path), nil

}
