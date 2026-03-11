package file

import (
	"fmt"
	"os"
	"path/filepath"
)

func TemporaryPath(app string, normal string) (string, error) {
	var path = filepath.Join(os.TempDir(), "mevpatch", app, normal)
	return filepath.Clean(path), nil
}

func PatcherPath() (string, error) {
	own, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Dir(own)), nil
}

func PersistentPath(app string, normal string) (string, error) {
	own, err := os.Executable()
	if err != nil {
		return "", err
	}
	var path string
	if app == "launcher" {
		path = filepath.Join(filepath.Dir(own), "..", "..", normal)
	} else {
		path = filepath.Join(filepath.Dir(own), "..", "..", app, normal)
	}
	return filepath.Clean(path), nil

}

func PatchBundlePath(app string, version string) (string, error) {
	var normal = filepath.Join("bin", fmt.Sprintf("%s_patch.bin", version))
	return TemporaryPath(app, normal)
}
