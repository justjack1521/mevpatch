package update

import (
	"archive/zip"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/database"
	"github.com/justjack1521/mevpatch/internal/patch"
	uuid "github.com/satori/go.uuid"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RemoteFilePatcher struct {
	application string
	version     string
	uri         string
	patchers    *RemoteFileMergeWorkerGroup
	committers  *FileMetadataCommitWorkerGroup
	errors      chan error
	remotes     map[uuid.UUID]*mevmanifest.File
}

func NewRemoteFilePatcher(app string, version string, download string, repo *database.PatchingRepository, remotes []*mevmanifest.File) *RemoteFilePatcher {
	var patcher = &RemoteFilePatcher{
		application: app,
		version:     version,
		uri:         download,
		patchers:    NewRemoteFileMergeWorkerGroup(app, 1),
		committers:  NewFileMetadataCommitWorkerGroup(app, repo, 1),
		errors:      make(chan error, 10),
	}
	patcher.remotes = make(map[uuid.UUID]*mevmanifest.File)
	for _, file := range remotes {
		id, err := uuid.FromString(file.Id)
		if err != nil {
			continue
		}
		patcher.remotes[id] = file
	}
	return patcher
}

func (u *RemoteFilePatcher) Start(merger *patch.Merger) error {

	u.patchers.Start(merger, u.committers.channel, u.errors)

	fmt.Println(fmt.Sprintf("[Remote File Patcher] Download Started %s", u.version))
	path, err := u.download()
	if err != nil {
		fmt.Println(fmt.Sprintf("[Remote File Patcher] Download Failed %s: %s", u.version, err))
		return err
	}
	fmt.Println(fmt.Sprintf("[Remote File Patcher] Download Complete %s", u.version))

	fmt.Println(fmt.Sprintf("[Remote File Patcher] Extract Started %s", u.version))
	if err := u.unzip(path); err != nil {
		fmt.Println(fmt.Sprintf("[Remote File Patcher] Extract Failed %s: %s", u.version, err))
		return err
	}
	fmt.Println(fmt.Sprintf("[Remote File Patcher] Extract Complete %s", u.version))

	close(u.patchers.channel)
	u.patchers.Wait()
	close(u.committers.channel)
	u.committers.Wait()
	close(u.errors)

	return nil

}

func (u *RemoteFilePatcher) unzip(path string) error {

	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	defer os.Remove(path)

	for _, file := range reader.File {

		var name = strings.TrimSuffix(file.Name, ".jdf")
		id, err := uuid.FromString(name)
		if err != nil {
			return err
		}
		remote, exists := u.remotes[id]
		if exists == false {
			fmt.Println(fmt.Sprintf("not found: %s", id))
			continue
		}

		temp, err := TemporaryPath(u.application, filepath.Join("extract", file.Name))
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(temp), os.ModePerm); err != nil {
			return err
		}

		out, err := os.Create(temp)
		if err != nil {
			return err
		}
		defer out.Close()

		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		_, err = io.Copy(out, rc)
		if err != nil {
			return err
		}

		u.patchers.channel <- &RemoteFileMergeJob{
			FileID:            id,
			NormalPath:        remote.Path,
			PatchFileTempPath: temp,
		}

	}

	return nil

}

func (u *RemoteFilePatcher) download() (string, error) {
	response, err := http.Get(u.uri)
	if err != nil {
		return "", nil
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d for %s", response.StatusCode, u.download)
	}

	path, err := TemporaryPath(u.application, filepath.Join("bin", fmt.Sprintf("%s_patch.bin", u.version)))
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return "", err
	}

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, response.Body)
	if err != nil {
		return "", err
	}

	return path, nil

}
