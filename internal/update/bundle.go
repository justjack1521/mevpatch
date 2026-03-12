package update

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/patch"
	uuid "github.com/satori/go.uuid"
)

// BundleDownloader downloads and extracts the patch bundle for the current version.
type BundleDownloader struct {
	app string
	// remotes maps file UUID → manifest file, for all files that need patching.
	remotes map[uuid.UUID]*mevmanifest.File
}

func NewBundleDownloader(app string, plan *Plan) *BundleDownloader {
	d := &BundleDownloader{
		app:     app,
		remotes: make(map[uuid.UUID]*mevmanifest.File),
	}
	for _, f := range plan.FilesRequirePatch {
		id, err := uuid.FromString(f.Id)
		if err != nil || uuid.Equal(id, uuid.Nil) {
			continue
		}
		d.remotes[id] = f
	}
	return d
}

// Download fetches the bundle zip from the remote and saves it to a temp path.
// progress receives byte-count increments during the download.
func (d *BundleDownloader) Download(targetVersion patch.Version, bundle *mevmanifest.Bundle, progress chan<- float32) error {
	defer close(progress)

	destination := file.PatchBundlePath(d.app, targetVersion.String())
	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return fmt.Errorf("creating bundle dir: %w", err)
	}

	resp, err := http.Get(bundle.DownloadPath)
	if err != nil {
		return fmt.Errorf("downloading bundle from %s: %w", bundle.DownloadPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading bundle from %s", resp.StatusCode, bundle.DownloadPath)
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("creating bundle file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("writing bundle: %w", writeErr)
			}
			progress <- float32(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("reading bundle response: %w", readErr)
		}
	}

	return nil
}

// Unzip extracts the bundle, verifies each entry, and returns merge jobs.
// done receives one signal per successfully extracted file.
func (d *BundleDownloader) Unzip(currentVersion patch.Version, targetVersion patch.Version, done chan<- bool) ([]*patch.RemoteFileMergeJob, error) {
	defer close(done)

	destination := file.PatchBundlePath(d.app, targetVersion.String())
	defer os.Remove(destination)

	reader, err := zip.OpenReader(destination)
	if err != nil {
		return nil, fmt.Errorf("opening bundle zip: %w", err)
	}
	defer reader.Close()

	var results []*patch.RemoteFileMergeJob

	for _, entry := range reader.File {
		fmt.Printf("[Bundle] Extracting %s\n", entry.Name)

		// Entry names are "{uuid}.jdf" — strip suffix to get UUID.
		name := strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name))
		id, err := uuid.FromString(name)
		if err != nil {
			fmt.Printf("[Bundle] Skipping unrecognised entry: %s\n", entry.Name)
			continue
		}

		remote, exists := d.remotes[id]
		if !exists {
			fmt.Printf("[Bundle] Skipping entry not in plan: %s\n", entry.Name)
			continue
		}

		job, err := d.extractEntry(entry, remote, currentVersion)
		if err != nil {
			return nil, fmt.Errorf("extracting %s: %w", entry.Name, err)
		}

		results = append(results, job)
		done <- true
		fmt.Printf("[Bundle] Extracted %s → %s\n", entry.Name, remote.Path)
	}

	return results, nil
}

func (d *BundleDownloader) extractEntry(
	entry *zip.File,
	remote *mevmanifest.File,
	currentVersion patch.Version,
) (*patch.RemoteFileMergeJob, error) {
	// Find the PatchFile metadata for our current version inside the manifest entry.
	var patchMeta *mevmanifest.PatchFile
	for _, p := range remote.Patches {
		if p.Version == currentVersion.String() {
			patchMeta = p
			break
		}
	}
	if patchMeta == nil {
		return nil, fmt.Errorf("no patch metadata for version %s in file %s", currentVersion.String(), remote.Path)
	}

	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("opening zip entry: %w", err)
	}
	defer rc.Close()

	buf, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading zip entry: %w", err)
	}

	// Verify size and checksum of the extracted patch file.
	if int64(len(buf)) != patchMeta.Size {
		return nil, fmt.Errorf("patch file size mismatch: expected %d got %d", patchMeta.Size, len(buf))
	}
	checksum := patch.GetChecksumForBytes(buf)
	if checksum != patchMeta.Checksum {
		return nil, fmt.Errorf("patch file checksum mismatch for %s: expected %s got %s",
			remote.Path, patchMeta.Checksum, checksum)
	}

	// Write verified bytes to a temp path.
	extractPath := file.ExtractPath(d.app, entry.Name)
	if err := os.MkdirAll(filepath.Dir(extractPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating extract dir: %w", err)
	}
	if err := os.WriteFile(extractPath, buf, 0644); err != nil {
		return nil, fmt.Errorf("writing extracted patch: %w", err)
	}

	return &patch.RemoteFileMergeJob{
		ParentFile:        remote,
		PatchFileTempPath: extractPath,
	}, nil
}
