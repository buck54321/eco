// +build linux

package eco

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	decredPattern        = regexp.MustCompile(`^decred-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
	decreditonPattern    = regexp.MustCompile(`^decrediton-v(.*)\.tar\.gz$`)
	dexcPattern          = regexp.MustCompile(`^dexc-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.tar\.gz$`)
	programDirectory     = "/opt/decred-eco"
	dcrdExeName          = dcrd
	dcrWalletExeName     = dcrwallet
	decreditonExeName    = decrediton
	decreditonConfigPath = filepath.Join(osUser.HomeDir, ".config", "decrediton")
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
	archivePath, err := fetchAsset(ctx, tmpDir, asset)
	if err != nil {
		return fmt.Errorf("Error fetching %q: %w", asset.Name, err)
	}

	// Check the hash.
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("Error opening archive %q for hashing: %w", asset.Name, err)
	}
	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	f.Close()
	if err != nil {
		return fmt.Errorf("Error hashing archive %q: %w", asset.Name, err)
	}
	h := hasher.Sum(nil)
	if !bytes.Equal(h, checkHash) {
		return fmt.Errorf("File hash mismatch for %q. Expected %x, got %x", asset.Name, checkHash, h)
	}

	// Unpack.
	asset.path, err = unpack(archivePath)
	if err != nil {
		return fmt.Errorf("Error unpacking %q: %w", asset.Name, err)
	}
	return nil
}
