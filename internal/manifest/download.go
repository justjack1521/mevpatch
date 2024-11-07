package manifest

import (
	"encoding/json"
	"fmt"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/patch"
	"io"
	"net/http"
	"net/url"
)

func DownloadManifest(host string, app string, version patch.Version) (*mevmanifest.Manifest, error) {

	uri, err := url.JoinPath(host, "downloads", app, "manifest", fmt.Sprintf("%s_manifest.json", version.String()))
	if err != nil {
		return nil, err
	}

	fmt.Println(uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result = &mevmanifest.Manifest{}
	if err := json.Unmarshal(body, result); err != nil {
		return nil, err
	}

	return result, nil

}
