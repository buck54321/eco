// +build linux

package eco

import (
	"regexp"
	"runtime"
)

var (
	serverAddress     = &NetAddr{"tcp4", TCPSocketHost}
	decredPattern     = regexp.MustCompile(`^decred-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.zip$`)
	decreditonPattern = regexp.MustCompile(`^decrediton-v(.*)\.exe$`)
	dexcPattern       = regexp.MustCompile(`^dexc-` + runtime.GOOS + `-` + runtime.GOARCH + `-v(.*)\.zip$`)

	// Decrediton JS for home direcotry for Windows and Darwin
	// if (os.platform() == "win32") {
	// 	return path.join(os.homedir(), "AppData", "Local", "Decrediton");
	//   } else if (process.platform === "darwin") {
	// 	return path.join(os.homedir(), "Library","Application Support","decrediton");
	//   }
)
