package eco

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unzip(src string) (string, error) {
	dest := filepath.Dir(src)
	var topDir string
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Errorf("Error closing zip reader: %v", err)
		}
	}()

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				log.Errorf("Error closing file %q after extraction: %v", f.Name, err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		isDir := f.FileInfo().IsDir()
		dirPath := path
		if !isDir {
			dirPath = filepath.Dir(path)
		}
		err = os.MkdirAll(dirPath, f.Mode())
		if err != nil {
			return err
		}

		if isDir {
			if topDir == "" {
				topDir = dirPath
			}
			return nil
		}
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer func() {
			if err := outFile.Close(); err != nil {
				log.Errorf("Error closing extracted file %q: %v", path, err)
			}
		}()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return "", err
		}
	}

	return topDir, nil
}
