package eco

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"os/exec"
	"path/filepath"
	"regexp"

	walletclient "decred.org/dcrwallet/rpc/client/dcrwallet"
	"github.com/decred/dcrd/rpcclient/v6"
	"github.com/decred/slog"
)

// Services
const (
	decred     = "decred"
	dcrd       = "dcrd"
	dcrwallet  = "dcrwallet"
	dexc       = "dexc"
	decrediton = "decrediton"
	dcrctl     = "dcrctl"

	minChromiumMajorVersion = 76
)

var (
	manifestPattern  = regexp.MustCompile(`^.*-manifest\.txt$`)
	dcrdAppDir       = filepath.Join(AppDir, dcrd)
	dcrdCertPath     = filepath.Join(AppDir, dcrd, "rpc.cert")
	dcrwalletAppDir  = filepath.Join(AppDir, dcrwallet)
	dcrWalletRPCCert = filepath.Join(dcrwalletAppDir, "rpc.cert")
	dcrWalletRPCKey  = filepath.Join(dcrwalletAppDir, "rpc.key")
	decreditonAppDir = filepath.Join(AppDir, decrediton)
	dexAppDir        = filepath.Join(AppDir, dexc)

	dcrdRPCListen       = ":19703"
	dcrdListen          = ":19704"
	dcrWalletRPCListen  = ":19705"
	dcrWalletGRPCListen = ":19706"
	dexWebAddr          = ":26270"

	chromiumVersionRegexp = regexp.MustCompile(`^[^\d]*(\d+)\.(\d+)\.(\d+)`)

	dexAcctName = "dex"
)

type serviceExe struct {
	name   string
	cmd    *exec.Cmd
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	feed   func([]byte)
}

func newExe(ctx context.Context, exe string, args ...string) *serviceExe {
	exeCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(exeCtx, exe, args...)
	cmd.Dir = filepath.Dir(exe)

	s := &serviceExe{
		name:   filepath.Base(exe),
		cmd:    cmd,
		ctx:    exeCtx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	cmd.Stdout = &outputWriter{s.processOutput}

	return s
}

func (s *serviceExe) processOutput(msg []byte) {
	// For dcrwallet, there is not a great indicator of sync status via RPC.
	// We could potentially catch the output during startup and read it here.
	// The sync line ends like below.
	//
	// Blockchain sync completed, wallet ready for general usage
	if s.feed != nil {
		s.feed(msg)
	}
}

// Run will run the service repeatedly until the service Context is canceled.
func (s *serviceExe) Run() error {
	defer close(s.done)
	log.Infof("Running %q", s.cmd)
	err := s.cmd.Run()
	log.Tracef("%s finished", s.name)
	if err != nil && s.ctx.Err() == nil {
		log.Errorf("Error encountered running %q: %v", s.cmd, err)
	}
	return err
}

// func (s *serviceExe) Wait() {
// 	<-s.done
// }

func (s *serviceExe) Done() <-chan struct{} {
	return s.done
}

type outputWriter struct {
	f func([]byte)
}

func (w *outputWriter) Write(p []byte) (n int, err error) {
	w.f(p)
	return len(p), nil
}

type executable struct {
	name, semver, archiveURL, archiveHash string
}

type DCRD struct {
	DCRDState
	client *rpcclient.Client
	exe    *serviceExe
}

func dcrdNewState() *DCRDState {
	return &DCRDState{
		RPCUser:      randomToken(),
		RPCPass:      randomToken(),
		UserSettings: dcrdDefaultUserSettings(),
	}
}

func dcrdDefaultUserSettings() DCRDUserSettings {
	return DCRDUserSettings{
		DebugLevel: slog.LevelDebug.String(),
	}
}

type DCRWallet struct {
	DCRWalletState
	client *walletclient.Client
	exe    *serviceExe
}

func dcrWalletNewState() *DCRWalletState {
	return &DCRWalletState{}
}

type decreditonConfig struct {
	Theme                 string             `json:"theme"`
	DaemonStartAdvanced   bool               `json:"daemon_start_advanced"`
	Locale                string             `json:"locale"`
	Network               string             `json:"network"`
	SetLanguage           bool               `json:"set_language"`
	UIAnimations          bool               `json:"ui_animations"`
	ShowTutorial          bool               `json:"show_tutorial"`
	ShowPrivacy           bool               `json:"show_privacy"`
	ShowSPVChoice         bool               `json:"show_spvchoice"`
	AllowExternalRequests []string           `json:"allowed_external_requests"`
	ProxyType             *string            `json:"proxy_type"`
	ProxyLocation         *string            `json:"proxy_location"`
	RemoteCredentials     *RemoteCredentials `json:"remote_credentials"`
	AppDataPath           string             `json:"appdata_path"`
	SPVMode               bool               `json:"spv_mode"`
	SPVConnect            []string           `json:"spv_connect"`
	MaxWalletCount        int                `json:"max_wallet_count"`
	Timezone              string             `json:"timezone"`
	DisableHardwareAccel  bool               `json:"disable_hardware_accel"`
	LastHeight            uint32             `json:"last_height"`
	TrezorDebug           bool               `json:"trezor_debug"`
	LNEnabled             bool               `json:"ln_enabled"`
	IsElectron8           bool               `json:"is_electron8"`
}

type RemoteCredentials struct {
	RPCUser string `json:"rpc_user"`
	RPCPass string `json:"rpc_pass"`
	RPCCert string `json:"rpc_cert"`
	RPCHost string `json:"rpc_host"`
	RPCPort string `json:"rpc_port"`
}

func defaultDecreditonConfig() *decreditonConfig {
	return &decreditonConfig{
		Theme:               "theme-dark",
		DaemonStartAdvanced: true,
		Locale:              "en",
		Network:             "mainnet",
		UIAnimations:        true,
		AllowExternalRequests: []string{
			"EXTERNALREQUEST_NETWORK_STATUS",
			"EXTERNALREQUEST_STAKEPOOL_LISTING",
			"EXTERNALREQUEST_UPDATE_CHECK",
			"EXTERNALREQUEST_DCRDATA",
			"EXTERNALREQUEST_POLITEIA",
		},
		AppDataPath:    decreditonAppDir,
		SPVConnect:     []string{},
		MaxWalletCount: 3,
		Timezone:       "local",
	}
}

const base58Chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func randomToken() string {
	sz := 16
	c := make([]byte, sz)
	for i := 0; i < sz; i++ {
		c[i] = base58Chars[int(randInt()%58)]
	}
	return string(c)
}

func randInt() uint64 {
	b := make([]byte, 8)
	rand.Read(b)
	return binary.BigEndian.Uint64(b)
}
