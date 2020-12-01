package eco

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/decred/slog"
)

func TestServer(t *testing.T) {
	log = slog.NewBackend(os.Stdout).Logger("TEST")
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	AppDir = tmpDir
	KeyPath = filepath.Join(tmpDir, "decred-eco.key")
	CertPath = filepath.Join(tmpDir, "decred-eco.cert")
	serverAddress = &NetAddr{
		Net:  "tcp4",
		Addr: ":39079",
	}
	dcrdState := dcrdNewState()
	runTest := func() {
		serverAddress = &NetAddr{
			Net:  "tcp4",
			Addr: ":39079",
		}
		srv, err := NewServer(&Eco{
			dcrd: &DCRD{DCRDState: *dcrdState},
		})
		if err != nil {
			t.Fatalf("NewServer error: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		go srv.Run(ctx)
		reState := new(DCRDState)
		err = serviceStatus(ctx, dcrd, reState)
		if err != nil {
			t.Fatalf("serviceStatus error: %v", err)
		}
		if reState.RPCPass != dcrdState.RPCPass {
			t.Fatalf("wrong AppDataDir decoded")
		}
	}
	runTest()

	if runtime.GOOS == "linux" {
		serverAddress = &NetAddr{
			Net:  "unix",
			Addr: filepath.Join(tmpDir, UnixSocketFilename),
		}
		runTest()
	}
}

func TestParseAssets(t *testing.T) {
	var release *githubRelease
	err := json.Unmarshal(testRelease, &release)
	if err != nil {
		t.Fatalf("JSON Marshal error: %v", err)
	}
	assets, err := parseAssets(release)
	if err != nil {
		t.Fatalf("parseAssets error: %v", err)
	}
	expDecred := "decred-" + runtime.GOOS + "-" + runtime.GOARCH + "-v1.6.0-rc3"
	if !strings.HasPrefix(assets.decred.Name, expDecred) {
		t.Fatalf("Wrong Decred chosen. Expected prefix %s, got %s", expDecred, assets.decred.Name)
	}
	expDecrediton := "decrediton-v1.6.0-rc3"
	if !strings.HasPrefix(assets.decrediton.Name, expDecrediton) {
		t.Fatalf("Wrong Decrediton chosen. Expected prefix %s, got %s", expDecrediton, assets.decrediton.Name)
	}
	expDexc := "dexc-" + runtime.GOOS + "-" + runtime.GOARCH + "-v0.1.2"
	if !strings.HasPrefix(assets.dexc.Name, expDexc) {
		t.Fatalf("Wrong DEX chosen. Expected prefix %s, got %s", expDexc, assets.dexc.Name)
	}
	if len(assets.manifests) != 3 {
		t.Fatalf("Manifest files not parsed")
	}
}

var testRelease = []byte(`{
	"name": "v1.6.0-rc3",
	"prerelease": true,
	"published_at": "2020-11-16T15:49:25Z",
	"assets": [
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28255098",
			"name": "decred-darwin-amd64-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 76481559,
			"download_count": 6,
			"created_at": "2020-11-12T22:53:13Z",
			"updated_at": "2020-11-12T22:55:47Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-darwin-amd64-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254954",
			"name": "decred-freebsd-amd64-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 77357373,
			"download_count": 0,
			"created_at": "2020-11-12T22:49:59Z",
			"updated_at": "2020-11-12T22:53:13Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-freebsd-amd64-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254913",
			"name": "decred-linux-386-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 75602847,
			"download_count": 2,
			"created_at": "2020-11-12T22:47:22Z",
			"updated_at": "2020-11-12T22:49:59Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-linux-386-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254864",
			"name": "decred-linux-amd64-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 77457144,
			"download_count": 60,
			"created_at": "2020-11-12T22:44:39Z",
			"updated_at": "2020-11-12T22:47:22Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-linux-amd64-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254702",
			"name": "decred-linux-arm-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 73035532,
			"download_count": 5,
			"created_at": "2020-11-12T22:39:36Z",
			"updated_at": "2020-11-12T22:42:04Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-linux-arm-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254768",
			"name": "decred-linux-arm64-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 72897712,
			"download_count": 2,
			"created_at": "2020-11-12T22:42:04Z",
			"updated_at": "2020-11-12T22:44:40Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-linux-arm64-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254572",
			"name": "decred-openbsd-amd64-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 77138258,
			"download_count": 1,
			"created_at": "2020-11-12T22:37:01Z",
			"updated_at": "2020-11-12T22:39:36Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-openbsd-amd64-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254571",
			"name": "decred-v1.6.0-rc3-manifest.txt",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "text/plain",
			"size": 924,
			"download_count": 146,
			"created_at": "2020-11-12T22:37:01Z",
			"updated_at": "2020-11-12T22:37:01Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-v1.6.0-rc3-manifest.txt",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28285503",
			"name": "decred-v1.6.0-rc3-manifest.txt.asc",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 833,
			"download_count": 138,
			"created_at": "2020-11-13T16:10:43Z",
			"updated_at": "2020-11-13T16:10:44Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-v1.6.0-rc3-manifest.txt.asc",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254470",
			"name": "decred-windows-386-v1.6.0-rc3.zip",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 76772294,
			"download_count": 7,
			"created_at": "2020-11-12T22:34:12Z",
			"updated_at": "2020-11-12T22:37:01Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-windows-386-v1.6.0-rc3.zip",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28254372",
			"name": "decred-windows-amd64-v1.6.0-rc3.zip",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 78089771,
			"download_count": 56,
			"created_at": "2020-11-12T22:31:28Z",
			"updated_at": "2020-11-12T22:34:12Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decred-windows-amd64-v1.6.0-rc3.zip",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28285582",
			"name": "decrediton-v1.6.0-rc3-manifest.txt",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "text/plain",
			"size": 276,
			"download_count": 7,
			"created_at": "2020-11-13T16:12:37Z",
			"updated_at": "2020-11-13T16:12:38Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decrediton-v1.6.0-rc3-manifest.txt",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28288223",
			"name": "decrediton-v1.6.0-rc3-manifest.txt.asc",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 833,
			"download_count": 5,
			"created_at": "2020-11-13T17:18:19Z",
			"updated_at": "2020-11-13T17:18:19Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decrediton-v1.6.0-rc3-manifest.txt.asc",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28286092",
			"name": "decrediton-v1.6.0-rc3.dmg",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 198469445,
			"download_count": 12,
			"created_at": "2020-11-13T16:28:06Z",
			"updated_at": "2020-11-13T16:34:56Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decrediton-v1.6.0-rc3.dmg",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28285816",
			"name": "decrediton-v1.6.0-rc3.exe",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 229386824,
			"download_count": 37,
			"created_at": "2020-11-13T16:20:05Z",
			"updated_at": "2020-11-13T16:28:06Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decrediton-v1.6.0-rc3.exe",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28285583",
			"name": "decrediton-v1.6.0-rc3.tar.gz",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/gzip",
			"size": 205676068,
			"download_count": 23,
			"created_at": "2020-11-13T16:12:38Z",
			"updated_at": "2020-11-13T16:20:05Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/decrediton-v1.6.0-rc3.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282010",
			"name": "dexc-darwin-amd64-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 19341971,
			"download_count": 5,
			"created_at": "2020-11-13T14:39:14Z",
			"updated_at": "2020-11-13T14:39:41Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-darwin-amd64-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282027",
			"name": "dexc-freebsd-amd64-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 19585998,
			"download_count": 0,
			"created_at": "2020-11-13T14:39:42Z",
			"updated_at": "2020-11-13T14:40:09Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-freebsd-amd64-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282047",
			"name": "dexc-linux-386-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 19165284,
			"download_count": 1,
			"created_at": "2020-11-13T14:40:09Z",
			"updated_at": "2020-11-13T14:40:35Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-linux-386-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282060",
			"name": "dexc-linux-amd64-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 19591091,
			"download_count": 28,
			"created_at": "2020-11-13T14:40:35Z",
			"updated_at": "2020-11-13T14:41:04Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-linux-amd64-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282084",
			"name": "dexc-linux-arm-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 18532990,
			"download_count": 7,
			"created_at": "2020-11-13T14:41:30Z",
			"updated_at": "2020-11-13T14:41:56Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-linux-arm-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282070",
			"name": "dexc-linux-arm64-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 18467578,
			"download_count": 3,
			"created_at": "2020-11-13T14:41:04Z",
			"updated_at": "2020-11-13T14:41:30Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-linux-arm64-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282100",
			"name": "dexc-openbsd-amd64-v0.1.2.tar.gz",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/gzip",
			"size": 19532144,
			"download_count": 2,
			"created_at": "2020-11-13T14:41:56Z",
			"updated_at": "2020-11-13T14:42:24Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-openbsd-amd64-v0.1.2.tar.gz",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282123",
			"name": "dexc-v0.1.2-manifest.txt",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "text/plain",
			"size": 870,
			"download_count": 43,
			"created_at": "2020-11-13T14:42:24Z",
			"updated_at": "2020-11-13T14:42:25Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-v0.1.2-manifest.txt",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28285539",
			"name": "dexc-v0.1.2-manifest.txt.asc",
			"uploader": {
				"login": "dajohi",
				"avatar_url": "https://avatars0.githubusercontent.com/u/3308193?v=4"
			},
			"content_type": "application/octet-stream",
			"size": 833,
			"download_count": 42,
			"created_at": "2020-11-13T16:11:34Z",
			"updated_at": "2020-11-13T16:11:35Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-v0.1.2-manifest.txt.asc",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282124",
			"name": "dexc-windows-386-v0.1.2.zip",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/zip",
			"size": 19372117,
			"download_count": 0,
			"created_at": "2020-11-13T14:42:25Z",
			"updated_at": "2020-11-13T14:42:52Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-windows-386-v0.1.2.zip",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		},
		{
			"url": "https://api.github.com/repos/decred/decred-binaries/releases/assets/28282129",
			"name": "dexc-windows-amd64-v0.1.2.zip",
			"uploader": {
				"login": "jrick",
				"avatar_url": "https://avatars3.githubusercontent.com/u/1420313?v=4"
			},
			"content_type": "application/zip",
			"size": 19684489,
			"download_count": 7,
			"created_at": "2020-11-13T14:42:52Z",
			"updated_at": "2020-11-13T14:43:19Z",
			"browser_download_url": "https://github.com/decred/decred-binaries/releases/download/v1.6.0-rc3/dexc-windows-amd64-v0.1.2.zip",
			"author": {
				"login": "",
				"avatar_url": ""
			}
		}
	]
}`)

func TestFindChromiumBrowser(t *testing.T) {
	cmd, _, found, err := chromium(context.Background())
	if err != nil {
		t.Fatalf("error searching for chromium browser: %v", err)
	}
	if !found {
		t.Logf("no chromium browser found")
	}
	t.Logf("chromium browser found %q", cmd)
}
