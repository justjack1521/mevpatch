package manifest

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/justjack1521/mevpatch/internal/patch"
)

const versionHost = "https://mevius-mevway-9mxlj.ondigitalocean.app"

type patchRelease struct {
	Version string `json:"Version"`
}

// FetchCurrentVersion fetches the most recently released version for an application.
func FetchCurrentVersion(app string) (patch.Version, error) {
	url := fmt.Sprintf("%s/public/patch/recent?application=%s", versionHost, app)

	resp, err := http.Get(url)
	if err != nil {
		return patch.Version{}, fmt.Errorf("fetching current version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return patch.Version{}, fmt.Errorf("unexpected status %d fetching current version", resp.StatusCode)
	}

	var release patchRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return patch.Version{}, fmt.Errorf("decoding current version response: %w", err)
	}

	return patch.NewVersion(release.Version)
}
