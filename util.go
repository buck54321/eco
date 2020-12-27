package eco

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func ServerAddress() (string, string, error) {
	b, err := ioutil.ReadFile(filepath.Join(AppDir, ListenerFilename))
	if err != nil {
		return "", "", fmt.Errorf("Error reading server address file: %w", err)
	}
	s := string(b)
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid server address file format. Cannot read %q", s)
	}
	return parts[0], parts[1], nil
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
)

func downloadChromium(ctx context.Context, tmpDir, versionDir string, prog *progressReporter) error {
	downloadPath, checkHash := chromiumDownloadPath()
	if downloadPath == "" {
		return fmt.Errorf("no download path for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	prog.report(0, "Fetching Chromium")
	outPath, err := fetchAsset(ctx, tmpDir, downloadPath, "chromium.zip")
	if err != nil {
		return err
	}

	prog.report(0.75, "Validating Chromium download")
	err = checkFileHash(outPath, checkHash[:])
	if err != nil {
		return err
	}

	prog.report(0.80, "Extracting Chromium")
	unpacked, err := unzip(outPath)
	if err != nil {
		return fmt.Errorf("unzip error: %v", err)
	}

	return moveDirectoryContents(unpacked, filepath.Join(versionDir, "chromium"))
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
