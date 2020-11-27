// +build live

package eco

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

// go test -v -tags live -run FetchReleases
func TestFetchReleases(t *testing.T) {
	releases, err := fetchReleases()
	if err != nil {
		t.Fatalf("fetchReleases error: %v", err)
	}
	b, _ := json.MarshalIndent(releases[:5], "", "    ")
	fmt.Println(string(b))
}

// go test -v -tags live -run FetchAsset
func TestFetchAsset(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpDir)
	asset := &releaseAsset{
		githubAsset: &githubAsset{
			Name: "decred-v1.6.0-rc3-manifest.txt",
			URL:  "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254571",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	path, err := fetchAsset(ctx, tmpDir, asset)
	if err != nil {
		t.Fatalf("fetchAsset error: %v", err)
	}
	if !fileExists(path) {
		t.Fatalf("no file where fetchAsset reported")
	}
	b, _ := ioutil.ReadFile(path)
	content := string(b)
	if !strings.HasPrefix(content, "c33b26de3c5f2b24a5d423cbdc631405f591776596052e5cf5fd9669f3e5e5cf  decred-darwin-amd64-v1.6.0-rc3.tar.gz") {
		t.Fatalf("Wrong file contents")
	}
}
