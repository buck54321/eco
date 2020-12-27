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
	"strings"

	"github.com/decred/dcrd/dcrutil"
)

var (
	EcoDir = filepath.Join(osUser.HomeDir, ".local", "share", "decred-eco") // "/opt/decred-eco"
	AppDir = dcrutil.AppDataDir("decred-eco", false)

	serverAddress             = &NetAddr{"unix", filepath.Join(AppDir, UnixSocketFilename)}
	decredPattern             = regexp.MustCompile(`^decred-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
	decreditonPattern         = regexp.MustCompile(`^decrediton-v(.*)\.tar\.gz$`)
	dexcPattern               = regexp.MustCompile(`^dexc-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
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

	topDir, err := ExtractTarGz(dir, archive)
	if err != nil {
		return "", fmt.Errorf("Error extracting file: %v", err)
	}
	return filepath.Join(dir, topDir), nil
}

func ExtractTarGz(dir string, gzipStream io.Reader) (string, error) {
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
			outFile, err := os.OpenFile(filepath.Join(dir, header.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, guessPerms(header.Name))
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

func guessPerms(path string) os.FileMode {
	parts := strings.Split(filepath.Base(path), ".")
	if len(parts) < 2 {
		return 0755
	}
	return 0644
}

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func Tar(src string, w io.Writer) error {

	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		// return on any error
		if err != nil {
			return err
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.Replace(file, src, "", -1), string(filepath.Separator))

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
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

// fetchAndUnpack unpacks the archive and sets the asset.path.
func fetchAndUnpack(ctx context.Context, tmpDir string, asset *releaseAsset, hashes map[string][]byte, prog *progressReporter) error {
	checkHash, found := hashes[asset.Name]
	if !found {
		return fmt.Errorf("No hash in manifest for %s", asset.Name)
	}

	// Fetch the main asset.
	prog.report(0, "Fetching %s", asset.Name)
	archivePath, err := fetchAsset(ctx, tmpDir, asset.URL, asset.Name)
	if err != nil {
		return fmt.Errorf("Error fetching %q: %w", asset.Name, err)
	}

	// Check the hash.
	prog.report(0.73, "Validating %s", asset.Name)
	err = checkFileHash(archivePath, checkHash)
	if err != nil {
		return fmt.Errorf("Failed file hash check: %w", err)
	}

	// Unpack.
	prog.report(0.75, "Unpacking %s", asset.Name)
	asset.path, err = unpack(archivePath)
	if err != nil {
		return fmt.Errorf("Error unpacking %q: %w", asset.Name, err)
	}
	return nil
}

var (
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
	return "", nil, false, nil
}

func chromiumDownloadPath() (string, [32]byte) {
	if runtime.GOARCH == "amd64" {
		return chromiumLinux64Download, chromiumLinux64Hash
	}
	return "", [32]byte{}
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
