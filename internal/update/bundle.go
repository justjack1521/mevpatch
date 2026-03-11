package update

import (
	"archive/zip"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	uuid "github.com/satori/go.uuid"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	errFailedDownloadBundle = func(bundle *mevmanifest.Bundle, err error) error {
		return fmt.Errorf("failed to download patch bundle at %s: %w", bundle.Version, err)
	}
	errUnexpectedStatusCode = func(url string, code int) error {
		return fmt.Errorf("unexpected status code %s: %d", url, code)
	}
	errFailedUnzipBundle = func(err error) error {
		return fmt.Errorf("failed to unzip bundle: %w", err)
	}
	errFailedUnpackFile = func(name string, err error) error {
		return fmt.Errorf("failed to unpack patch file %s: %w", name, err)
	}
	errPatchFileNotFound = func(version patch.Version) error {
		return fmt.Errorf("patch file not found in manifest for version %s", version.String())
	}
	errPatchFileSizeMismatch = func(actual int64, expected int64) error {
		return fmt.Errorf("mismatch file size, expected %d got %d", expected, actual)
	}
	errPatchChecksumMismatch = func(actual string, expected string) error {
		return fmt.Errorf("mistmatch checksum, expected %s got %s", expected, actual)
	}
)

type BundleDownloader struct {
	app     string
	remotes map[uuid.UUID]*mevmanifest.File
}

func NewBundleDownloader(app string, plan *Plan) *BundleDownloader {
	var downloader = &BundleDownloader{
		app: app,
	}
	downloader.remotes = make(map[uuid.UUID]*mevmanifest.File)
	for _, file := range plan.FilesRequireDownload {
		id, err := uuid.FromString(file.Id)
		if err != nil || uuid.Equal(id, uuid.Nil) {
			continue
		}
		downloader.remotes[id] = file
	}
	for _, file := range plan.FilesRequirePatch {
		id, err := uuid.FromString(file.Id)
		if err != nil || uuid.Equal(id, uuid.Nil) {
			continue
		}
		downloader.remotes[id] = file
	}
	return downloader
}

func (d *BundleDownloader) unpack(current patch.Version, file *zip.File, remote *mevmanifest.File, done chan<- bool) (*patch.RemoteFileMergeJob, error) {

	var target *mevmanifest.PatchFile
	for _, child := range remote.Patches {
		if child.Version == current.String() {
			target = child
			break
		}
	}

	if target == nil {
		return nil, errFailedUnpackFile(file.Name, errPatchFileNotFound(current))
	}

	zf, err := file.Open()
	if err != nil {
		return nil, errFailedUnpackFile(file.Name, err)
	}
	defer zf.Close()

	var buffer []byte
	buffer, err = io.ReadAll(zf)
	if err != nil {
		return nil, errFailedUnpackFile(file.Name, err)
	}

	var size = int64(len(buffer))
	if size != target.Size {
		return nil, errFailedUnpackFile(file.Name, errPatchFileSizeMismatch(size, target.Size))
	}

	var checksum = patch.GetChecksumForBytes(buffer)
	if checksum != target.Checksum {
		return nil, errFailedUnpackFile(file.Name, errPatchChecksumMismatch(checksum, target.Checksum))
	}

	var path = d.path(d.app, filepath.Join("extract", file.Name))

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, errFailedUnpackFile(file.Name, err)
	}

	if err = os.WriteFile(path, buffer, 0664); err != nil {
		return nil, errFailedUnpackFile(file.Name, err)
	}

	done <- true

	return &patch.RemoteFileMergeJob{
		ParentFile:        remote,
		PatchFile:         target,
		PatchFileTempPath: path,
	}, nil

}

func (d *BundleDownloader) Unzip(current patch.Version, target patch.Version, done chan<- bool) ([]*patch.RemoteFileMergeJob, error) {

	defer close(done)

	destination, err := file.PatchBundlePath(d.app, target.String())
	if err != nil {
		return nil, errFailedUnzipBundle(err)
	}
	defer os.Remove(destination)

	reader, err := zip.OpenReader(destination)
	if err != nil {
		return nil, errFailedUnzipBundle(err)
	}
	defer reader.Close()

	var results = make([]*patch.RemoteFileMergeJob, 0)

	for _, f := range reader.File {

		fmt.Println(fmt.Sprintf("[File Unzip] Started %s", f.Name))

		var name = strings.TrimSuffix(f.Name, ".jdf")
		id, err := uuid.FromString(name)
		if err != nil {
			fmt.Println(fmt.Sprintf("[File Unzip] Skipped %s", f.Name))
			continue
		}

		remote, exists := d.remotes[id]
		if exists == false {
			fmt.Println(fmt.Sprintf("[File Unzip] Skipped %s", f.Name))
			continue
		}

		result, err := d.unpack(current, f, remote, done)
		if err != nil {
			fmt.Println(fmt.Sprintf("[File Unzip] Failed %s", f.Name))
			return nil, errFailedUnzipBundle(err)
		}

		results = append(results, result)
		fmt.Println(fmt.Sprintf("[File Unzip] Complete %s", f.Name))

	}

	return results, nil

}

func (d *BundleDownloader) Download(version patch.Version, bundle *mevmanifest.Bundle, done chan<- float32) error {

	defer close(done)

	destination, err := file.PatchBundlePath(d.app, version.String())
	if err != nil {
		return errFailedDownloadBundle(bundle, err)
	}

	response, err := http.Get(bundle.DownloadPath)
	if err != nil {
		return errFailedDownloadBundle(bundle, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errFailedDownloadBundle(bundle, errUnexpectedStatusCode(bundle.DownloadPath, response.StatusCode))
	}

	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return errFailedDownloadBundle(bundle, err)
	}

	out, err := os.Create(destination)
	if err != nil {
		return errFailedDownloadBundle(bundle, err)
	}
	defer out.Close()

	var buffer = make([]byte, 1024*16)
	for {
		next, err := response.Body.Read(buffer)
		if next > 0 {
			_, err := out.Write(buffer[:next])
			if err != nil {
				return err
			}
			done <- float32(next)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil

}

func (d *BundleDownloader) path(app string, normal string) string {
	return filepath.Clean(filepath.Join(os.TempDir(), "mevpatch", app, normal))
}
