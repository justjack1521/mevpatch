package database

import (
	"fmt"
	"github.com/justjack1521/mevpatch/internal/patch"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func DownloadDatabase(host string, app string, version patch.Version) error {

	uri, err := url.JoinPath(host, "downloads", "launcher", "database", fmt.Sprintf("%s.sqlite", "patching"))
	fmt.Println(uri)
	if err != nil {
		return err
	}

	path, err := databasePath(app)
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
