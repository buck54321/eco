package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type archiveWriter struct {
	f func([]byte) error
}

func (w *archiveWriter) Write(b []byte) (n int, err error) {
	return len(b), w.f(b)
}

func main() {
	f, err := os.OpenFile("archive.go", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("Error opening file for writing: %v \n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Fprint(f, "package main \n\nvar archive = []byte{")
	var n int

	archive := make([]byte, 0)

	err = TarGz("include", &archiveWriter{func(data []byte) error {
		for i := range data {
			if n%12 == 0 {
				fmt.Fprint(f, "\n\t")
			}
			n++
			archive = append(archive, data[i])
			_, err := fmt.Fprintf(f, "0x%02x, ", data[i])
			if err != nil {
				return err
			}
		}
		return nil
	}})

	if err != nil {
		fmt.Println("Error writing archive.go: %v", err)
		os.Exit(1)
	}

	fmt.Fprint(f, "\n}\n")
}

// TarGz takes a source and variable writers and walks 'source' writing each
// file found to the tar writer; the purpose for accepting multiple writers is
// to allow for multiple outputs (for example a file, or md5 hash)
//
// inspired by https://gist.github.com/sdomino/e6bc0c98f87843bc26bb
func TarGz(src string, writers ...io.Writer) error {

	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Error reading source directory %v", err)
	}

	mw := io.MultiWriter(writers...)

	gw := gzip.NewWriter(mw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		// return on any error
		if err != nil {
			return err
		}

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

		fmt.Println("--header.Name", header.Name)

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
