package eco

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrutil"
)

var AppDir = dcrutil.AppDataDir("decred-eco", false)

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
