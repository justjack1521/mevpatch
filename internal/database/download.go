package database

import (
	"github.com/justjack1521/mevpatch/internal/patch"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func DownloadDatabase(host string, app string, version patch.Version) error {

	uri, err := url.JoinPath(host, "downloads", app, "database", version.String(), "patching.sqlite")
	if err != nil {
		return err
	}

	path, err := databasePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	resp, err := http.Get(uri)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil

}
