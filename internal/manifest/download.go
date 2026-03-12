package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
)

func DownloadManifest(host string, app string, version patch.Version) (*mevmanifest.Manifest, error) {
	uri, err := url.JoinPath(host, "downloads", app, "manifest", fmt.Sprintf("%s_manifest.json", version.String()))
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching manifest from %s", resp.StatusCode, uri)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading manifest response: %w", err)
	}

	var result mevmanifest.Manifest
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing manifest JSON: %w", err)
	}

	return &result, nil
}
