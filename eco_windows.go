// +build windows

package eco

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/decred/dcrd/dcrutil"
)

var (
	serverAddress     = &NetAddr{"tcp4", TCPSocketHost}
	decredPattern     = regexp.MustCompile(`^decred-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.zip$`)
	decreditonPattern = regexp.MustCompile(`^decrediton-v(.*)\.exe$`)
	dexcPattern       = regexp.MustCompile(`^dexc-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.zip$`)

	EcoDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "DecredEco")
	AppDir = dcrutil.AppDataDir("DecredEco", false)

	// Decrediton JS for home direcotry for Windows and Darwin
	// if (os.platform() == "win32") {
	// 	return path.join(os.homedir(), "AppData", "Local", "Decrediton");
	//   } else if (process.platform === "darwin") {
	// 	return path.join(os.homedir(), "Library","Application Support","decrediton");
	//   }
)

func moveResources(ctx context.Context, tmpDir string, assets *releaseAssets, hashes map[string][]byte, prog *progressReporter) error {
	versionDir := filepath.Join(EcoDir, assets.version)
	err := os.MkdirAll(versionDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating version directory: %w", err)
	}

	fetchMove := func(subDir string, asset *releaseAsset, prog *progressReporter) error {
		err = fetchAndUnpack(ctx, tmpDir, asset, hashes, prog.subReporter(0, 0.9))
		if err != nil {
			return fmt.Errorf("Archive retrieval error: %w", err)
		}

		prog.report(0.9, "Installing %s", asset.Name)
		tgt := filepath.Join(versionDir, subDir)
		err = moveDirectoryContents(asset.path, tgt)
		if err != nil {
			return fmt.Errorf("Error moving contents from %s to %s: %v", asset.path, tgt, err)
		}
		return nil
	}

	err = fetchMove(decred, assets.decred, prog.subReporter(0.25, 0.60))
	if err != nil {
		return fmt.Errorf("Decred program files: %w", err)
	}

	err = fetchMove(decrediton, assets.decrediton, prog.subReporter(0.60, 0.85))
	if err != nil {
		return fmt.Errorf("Decrediton files: %w", err)
	}

	// Delete the bin directory that ships with Decrediton. We'll point it to
	// the main Decred directory.
	err = os.RemoveAll(filepath.Join(versionDir, decrediton, "resources", "bin"))
	if err != nil {
		log.Errorf("Error removing Decrediton bin directory: %v", err)
	}

	err = fetchMove(dexc, assets.dexc, prog.subReporter(0.85, 1))
	if err != nil {
		return fmt.Errorf("DEX files: %w", err)
	}

	return nil
}

func chromium(ctx context.Context) (path string, args []string, found bool, err error) {
	roots := []string{
		os.Getenv("CommonProgramFiles(x86)"),
		os.Getenv("LOCALAPPDATA"),
	}

	subPaths := []string{
		filepath.Join("Chromium", "Application", "chrome.exe"), // chromium
		filepath.Join("BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
	}

	search := func(root string) (string, bool) {
		for _, subPath := range subPaths {
			exe := filepath.Join(root, subPath)
			if !fileExists(exe) {
				continue
			}

			// For Windows, the --version flag is apparently innefective.
			// https://bugs.chromium.org/p/chromium/issues/detail?id=158372
			// But the version is the name of a sub-directory.
			items, _ := ioutil.ReadDir(filepath.Dir(exe))
			for _, item := range items {
				if !item.IsDir() {
					continue
				}
				major, _, _, found := parseVersion(item.Name())
				if found && major > minChromiumMajorVersion {
					return exe, true
				}
			}
		}
		return "", false
	}

	for _, root := range roots {
		exe, found := search(root)
		if found {
			return exe, chromiumFlags, true, nil
		}
	}

	// If the browser is not found, we could download it from e.g.
	// https://www.googleapis.com/download/storage/v1/b/chromium-browser-snapshots/o/Win_x64%2F818425%2Fchrome-win.zip?generation=1603109374298105&alt=media
	// and unzip it to the .dexc directory.

	return "", nil, false, nil //fmt.Errorf("No browser found. Install Chromium, Brave, or Chrome to run the Decred DEX Client GUI")
}
