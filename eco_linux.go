// +build linux

package eco

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var (
	serverAddress             = &NetAddr{"unix", filepath.Join(AppDir, UnixSocketFilename)}
	decredPattern             = regexp.MustCompile(`^decred-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
	decreditonPattern         = regexp.MustCompile(`^decrediton-v(.*)\.tar\.gz$`)
	dexcPattern               = regexp.MustCompile(`^dexc-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
	programDirectory          = "/opt/decred-eco"
	dcrdExeName               = dcrd
	dcrWalletExeName          = dcrwallet
	decreditonExeName         = decrediton
	decreditonConfigPath      = filepath.Join(osUser.HomeDir, ".config", "decrediton")
	selfInstalledChromiumPath = filepath.Join(AppDir, "chromium", "chromium-browser")
	dexcExeName               = dexc

	chromiumLinux64Download = "https://www.googleapis.com/download/storage/v1/b/chromium-browser-snapshots/o/Linux_x64%2F831895%2Fchrome-linux.zip?generation=1606761066330041&alt=media"
	chromiumLinux64Hash     = [32]byte{
		0x38, 0x08, 0xd6, 0x80, 0xd4, 0x87, 0x72, 0xc5,
		0xdb, 0x1f, 0x15, 0x7c, 0xba, 0x1a, 0x6c, 0xc6,
		0xd6, 0x8e, 0x44, 0x64, 0xf4, 0x29, 0x22, 0x13,
		0x94, 0xa7, 0x1a, 0x98, 0xea, 0xf5, 0x37, 0xe8,
	}
)

func unpack(archivePath string) (string, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("Error opening archive: %w", err)
	}
	if !strings.HasSuffix(archivePath, ".tar.gz") {
		return "", fmt.Errorf("Expected tar.gz. Saw %q", archivePath)
	}
	dir := filepath.Dir(archivePath)

	// fmt.Println("--dir", dir, "archivePath", archivePath)

	// if err := os.Mkdir(dir, 0755); err != nil {
	// 	return "", fmt.Errorf("Mkdir() failed: %w", err)
	// }
	topDir, err := extractTarGz(dir, archive)
	if err != nil {
		return "", fmt.Errorf("Error extracting file: %v", err)
	}
	return filepath.Join(dir, topDir), nil
}

func extractTarGz(dir string, gzipStream io.Reader) (string, error) {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return "", fmt.Errorf("ExtractTarGz: NewReader error: %w", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	var topDir string

	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("ExtractTarGz: Next() failed: %w", err)
		}

		log.Infof("Extracting %s", header.Name)

		subDir := filepath.Join(dir, filepath.Dir(header.Name))
		if header.Typeflag == tar.TypeDir {
			if topDir == "" {
				topDir = header.Name
			}
			subDir = filepath.Join(dir, header.Name)
		}
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return "", fmt.Errorf("ExtractTarGz: MkdirAll() failed: %w", err)
		}
		if header.Typeflag == tar.TypeReg {
			outFile, err := os.OpenFile(filepath.Join(dir, header.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				return "", fmt.Errorf("ExtractTarGz: OpenFile() failed: %w", err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return "", fmt.Errorf("ExtractTarGz: Copy() failed: %w", err)
			}
			outFile.Close()
		}
	}
	return topDir, nil
}

func moveDirectoryContents(fromDir, toDir string) error {
	return filepath.Walk(fromDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("filepath.Walk error: %v", err)
			return nil
		}
		if info.IsDir() {
			dir := filepath.Join(toDir, strings.TrimPrefix(path, fromDir))
			log.Infof("Creating directory %q", dir)
			return os.MkdirAll(dir, 0755)
		}
		// Skip sample .conf files.
		if strings.HasSuffix(info.Name(), ".conf") {
			log.Infof("Skipping sample config file %q", info.Name())
			return nil
		}

		// TODO: Assuming same drive here. Probably need to be smarter.
		toPath := filepath.Join(toDir, strings.TrimPrefix(path, fromDir))
		log.Infof("Moving unpacked file from %q to %q", path, toPath)
		return os.Rename(path, toPath)
	})
}

func moveResources(ctx context.Context, tmpDir string, assets *releaseAssets, hashes map[string][]byte, report func(float32, string, ...interface{})) error {
	versionDir := filepath.Join(programDirectory, assets.version)
	err := os.MkdirAll(versionDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating version directory: %w", err)
	}

	fetchMove := func(subDir string, asset *releaseAsset) error {
		err = fetchAndUnpack(ctx, tmpDir, asset, hashes)
		if err != nil {
			return fmt.Errorf("Archive retrieval error: %w", err)
		}

		tgt := filepath.Join(versionDir, subDir)
		err = moveDirectoryContents(asset.path, tgt)
		if err != nil {
			return fmt.Errorf("Error moving contents from %s to %s: %v", asset.path, tgt, err)
		}
		return nil
	}

	report(0.25, "Installing program files for %s", assets.version)
	err = fetchMove(decred, assets.decred)
	if err != nil {
		return fmt.Errorf("Decred program files: %w", err)
	}

	report(0.60, "Installing Decrediton %s", assets.version)
	err = fetchMove(decrediton, assets.decrediton)
	if err != nil {
		return fmt.Errorf("Decrediton files: %w", err)
	}

	// Delete the bin directory that ships with Decrediton. We'll point it to
	// the main Decred directory.
	err = os.RemoveAll(filepath.Join(versionDir, decrediton, "resources", "bin"))
	if err != nil {
		log.Errorf("Error removing Decrediton bin directory: %v", err)
	}

	report(0.85, "Installing DEX %s", assets.version)
	err = fetchMove(dexc, assets.dexc)
	if err != nil {
		return fmt.Errorf("DEX files: %w", err)
	}

	return nil
}

// fetchAndUnpack unpacks the archive and sets the asset.path.
func fetchAndUnpack(ctx context.Context, tmpDir string, asset *releaseAsset, hashes map[string][]byte) error {
	checkHash, found := hashes[asset.Name]
	if !found {
		return fmt.Errorf("No hash in manifest for %s", asset.Name)
	}

	// Fetch the main asset.
	archivePath, err := fetchAsset(ctx, tmpDir, asset.URL, asset.Name)
	if err != nil {
		return fmt.Errorf("Error fetching %q: %w", asset.Name, err)
	}

	// Check the hash.
	err = checkFileHash(archivePath, checkHash)
	if err != nil {
		return fmt.Errorf("Failed file hash check: %w", err)
	}

	// Unpack.
	asset.path, err = unpack(archivePath)
	if err != nil {
		return fmt.Errorf("Error unpacking %q: %w", asset.Name, err)
	}
	return nil
}

var (
	chromiumDir     = filepath.Join(AppDir, "chromium")
	chromiumDataDir = filepath.Join(chromiumDir, "appdata")
	chromiumFlags   = []string{
		"--user-data-dir=" + chromiumDataDir,
		"--disable-extensions",
		"--no-first-run",
		"--app=http://localhost" + dexWebAddr,
	}
	linuxCmds = [4]string{"chromium-browser", "brave-browser", "google-chrome"}
)

// chromium attempts to find a suitable Chromium-based browser. The
// browser executable is assumed to be in PATH, and one of linuxCmds.
func chromium(ctx context.Context) (path string, args []string, found bool, err error) {
	err = os.MkdirAll(chromiumDataDir, 0700)
	if err != nil {
		return "", nil, false, fmt.Errorf("MkDirAll error for directory %s", chromiumDataDir)
	}
	for _, cmd := range linuxCmds {
		major, _, _, found := getBrowserVersion(ctx, cmd)
		if found && major >= minChromiumMajorVersion {
			return cmd, chromiumFlags, true, nil
		}
	}
	// Check if we have installed it ourselves.
	if fileExists(selfInstalledChromiumPath) {
		return selfInstalledChromiumPath, chromiumFlags, true, nil
	}
	// Edge is chromium-based now, so if we can figure out how to check the
	// version and start it with the requisite flags, that should definitely be
	// added here.
	return "", nil, false, nil
}

func chromiumDownloadPath() (string, [32]byte) {
	if runtime.GOARCH == "amd64" {
		return chromiumLinux64Download, chromiumLinux64Hash
	}
	return "", [32]byte{}
}

func downloadChromium(ctx context.Context, tmpDir, versionDir string) error {
	downloadPath, checkHash := chromiumDownloadPath()
	if downloadPath == "" {
		return fmt.Errorf("no download path for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	outPath, err := fetchAsset(ctx, tmpDir, downloadPath, "chromium.zip")
	if err != nil {
		return err
	}

	err = checkFileHash(outPath, checkHash[:])
	if err != nil {
		return err
	}

	unpacked, err := unzip(outPath)
	if err != nil {
		return fmt.Errorf("unzip error: %v", err)
	}

	return moveDirectoryContents(unpacked, filepath.Join(versionDir, "chromium"))
}

// Get browser version attempts to get the currently installed version for the
// specified browser executable. The boolean return value, found, indicates if
// a browser with an acceptable version is located.
func getBrowserVersion(ctx context.Context, cmd string) (major, minor, patch int, found bool) {
	cmdOut, err := exec.CommandContext(ctx, cmd, "--version").Output()
	if err == nil {
		log.Tracef("%s has version %s\n", cmd, string(cmdOut))
		return parseVersion(string(cmdOut))
	}
	return
}

func parseVersion(ver string) (major, minor, patch int, found bool) {
	matches := chromiumVersionRegexp.FindStringSubmatch(ver)
	if len(matches) == 4 {
		// The regex grouped on \d+, so an error is impossible(?).
		major, _ = strconv.Atoi(matches[1])
		minor, _ = strconv.Atoi(matches[2])
		patch, _ = strconv.Atoi(matches[3])
		found = true
	}
	return
}
