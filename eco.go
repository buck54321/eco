package eco

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	walletclient "decred.org/dcrwallet/rpc/client/dcrwallet"
	wallettypes "decred.org/dcrwallet/rpc/jsonrpc/types"
	"github.com/buck54321/eco/db"
	"github.com/buck54321/eco/encode"
	"github.com/buck54321/eco/encrypt"
	"github.com/decred/dcrd/chaincfg/v3"
	chainjson "github.com/decred/dcrd/rpc/jsonrpc/types/v2"
	"github.com/decred/dcrd/rpcclient/v6"
	"golang.org/x/net/publicsuffix"
)

const (
	UnixSocketFilename = "decred.sock"
	TCPSocketHost      = ":45219"
	ListenerFilename   = "addr.txt"
	dbFilename         = "eco.db"

	crypterKey    = "crypter"
	walletSeedKey = "walletSeed"
	extraInputKey = "extraInput"
	dexInputKey   = "dexInput"
	ecoStateKey   = "ecoState"
)

var (
	KeyPath  = filepath.Join(AppDir, "decred-eco.key")
	CertPath = filepath.Join(AppDir, "decred-eco.cert")

	dcrdRunning, dcrdSyncedOnce, dcrWalletRunning,
	decreditonRunning, dcrwalletRunningOnce, dexRunning,
	dexWindowOpen uint32

	osUser, _ = user.Current()
)

type NetAddr struct {
	Net  string
	Addr string
}

type Eco struct {
	db             *db.DB
	innerCtx       context.Context
	outerCtx       context.Context
	dcrdSynced     chan struct{}
	dcrwalletReady chan struct{}

	syncMtx   sync.Mutex
	syncCache map[string]*FeedMessage
	syncChans map[chan *FeedMessage]struct{}

	stateMtx   sync.RWMutex
	state      MetaState
	versionDir string
	dcrd       *DCRD
	dcrwallet  *DCRWallet
}

func Run(outerCtx context.Context) {
	// Create the app directory
	err := os.MkdirAll(AppDir, 0755)
	if err != nil {
		log.Errorf("Error creating application directory: %w", err)
		return
	}

	dbPath := filepath.Join(AppDir, dbFilename)
	dbb, err := db.NewDB(dbPath, backendLog.Logger("DB"))
	if err != nil {
		log.Errorf("Error creating database: %w", err)
		return
	}

	var state *EcoState
	_, err = dbb.FetchDecode(ecoStateKey, &state)
	// failure to load here is not an error.
	if err != nil {
		log.Errorf("Eco State load error: %v", err)
		return
	}
	// If the state is nil with no error, Eco is uninitialized.
	var dcrdState *DCRDState
	var dcrWalletState *DCRWalletState
	if state == nil {
		state = &EcoState{
			SyncMode: SyncModeUninitialized,
		}
		dcrdState = dcrdNewState()

		fmt.Println("--storing eco state", dirtyEncode(state))

		dbb.EncodeStore(ecoStateKey, state)
		dbb.EncodeStore(svcKey(dcrd), dcrdState)
		dcrWalletState = dcrWalletNewState()
		dbb.EncodeStore(svcKey(dcrwallet), dcrWalletState)
	} else {
		loadService := func(svc string, state interface{}) bool {
			loaded, err := dbb.FetchDecode(svcKey(svc), state)
			if err != nil {
				log.Errorf("Error loading %s state: %v", svc, err)
				return false
			}
			if !loaded {
				log.Errorf("No %s state in database. This shouldn't happen.", svc)
				return false
			}
			return true
		}
		if !loadService(dcrd, &dcrdState) {
			return
		}
		if !loadService(dcrwallet, &dcrWalletState) {
			return
		}
	}

	// Populate the WalletExists field so the GUI knows whether to prompt for
	// a password.
	state.WalletExists = walletFileExists()

	// We need an inner Context that is delayed on cancellation to allow clean
	// shutdown of e.g. dcrd
	innerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	services := make(map[string]*ServiceStatus, 2)
	services[decrediton] = &ServiceStatus{Service: decrediton}

	dexNeedsInit, _ := dbb.FetchDecode(dexInputKey, new(pwCache))
	if !dexNeedsInit {
		services[dexc] = &ServiceStatus{Service: dexc}
	}

	eco := &Eco{
		db:       dbb,
		innerCtx: innerCtx,
		outerCtx: outerCtx,
		state: MetaState{
			Eco:      *state,
			Services: map[string]*ServiceStatus{},
		},
		versionDir:     filepath.Join(EcoDir, state.Version),
		dcrd:           &DCRD{DCRDState: *dcrdState},
		dcrwallet:      &DCRWallet{DCRWalletState: *dcrWalletState},
		syncChans:      make(map[chan *FeedMessage]struct{}),
		dcrdSynced:     make(chan struct{}),
		dcrwalletReady: make(chan struct{}),
		syncCache:      make(map[string]*FeedMessage),
	}

	go func() {
		<-outerCtx.Done()
		time.AfterFunc(time.Second*30, func() { cancel() })
		err := eco.stopDCRD()
		if err != nil {
			log.Errorf("Error closing dcrd: %v", err)
		}
		err = eco.stopDCRWallet()
		if err != nil {
			log.Errorf("Error closing dcrwallet: %v", err)
		}
		cancel()
	}()

	if state.SyncMode != SyncModeUninitialized {
		eco.state.Services[decrediton] = &ServiceStatus{Service: decrediton}
		eco.start()
	}

	for {
		srv, err := NewServer(eco)
		if err == nil {
			srv.Run(outerCtx)
		} else {
			// If we didn't even create the server, something is desperately
			// wrong.
			log.Errorf("Error creating Eco server: %v", err)
			break
		}

		if outerCtx.Err() != nil {
			break
		}
		// We only get here with an unkown server error. Wait a second and loop
		// again.
		time.Sleep(time.Second)
	}
	<-innerCtx.Done()
}

func (eco *Eco) start() {
	if eco.state.Eco.SyncMode == SyncModeFull {
		err := eco.runDCRD()
		if err != nil {
			log.Errorf("dcrd startup error: %w", err)
		}
	}
	err := eco.runDCRWallet()
	if err != nil {
		log.Errorf("dcrwallet startup error: %w", err)
	}

	// The dexInputKey is only stored until initialized.
	if dexNeedsInit, _ := eco.db.FetchDecode(dexInputKey, new(pwCache)); dexNeedsInit {
		err := eco.runDEX()
		if err != nil {
			log.Errorf("DEX initialization error: %v", err)
		}
	}
}

func (eco *Eco) dcrdClient() (*rpcclient.Client, error) {
	return eco.newRPCClient(dcrdRPCListen, dcrdCertPath)
}

func (eco *Eco) dcrWalletClient() (*walletclient.Client, error) {
	cl, err := eco.newRPCClient(dcrWalletRPCListen, dcrWalletRPCCert)
	if err != nil {
		return nil, err
	}
	return walletclient.NewClient(walletclient.RawRequestCaller(cl), chaincfg.MainNetParams()), nil
}

func (eco *Eco) newRPCClient(rpcListen, certPath string) (*rpcclient.Client, error) {
	eco.stateMtx.RLock()
	rpcUser, rpcPass := eco.dcrd.RPCUser, eco.dcrd.RPCPass
	eco.stateMtx.RUnlock()
	certs, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("TLS certificate read error: %v", err)
	}

	config := &rpcclient.ConnConfig{
		Host:         "localhost" + rpcListen,
		HTTPPostMode: true,
		User:         rpcUser,
		Pass:         rpcPass,
		Certificates: certs,
	}

	return rpcclient.New(config, nil)
}

func (eco *Eco) dcrdState() (cfg *DCRDState) {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	sCopy := eco.dcrd.DCRDState
	return &sCopy
}

func (eco *Eco) metaState() (cfg *MetaState) {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	sCopy := eco.state
	return &sCopy
}

func (eco *Eco) syncMode() SyncMode {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	return eco.state.Eco.SyncMode
}

func (eco *Eco) syncChan() chan *FeedMessage {
	eco.syncMtx.Lock()
	ch := make(chan *FeedMessage, len(eco.syncCache))
	eco.syncChans[ch] = struct{}{}
	for _, u := range eco.syncCache {
		ch <- u
	}
	eco.syncMtx.Unlock()
	return ch
}

func (eco *Eco) returnSyncChan(ch chan *FeedMessage) {
	eco.syncMtx.Lock()
	delete(eco.syncChans, ch)
	eco.syncMtx.Unlock()
}

func (eco *Eco) sendSyncUpdate(pu *Progress) {

	fmt.Println("--sendSyncUpdate", dirtyEncode(pu))

	eco.syncMtx.Lock()
	defer eco.syncMtx.Unlock()
	st := eco.state.Services[pu.Service]
	if st != nil {
		st.Sync = pu
	}
	eco.sendFeedMessage(syncKey(pu.Service), MsgTypeSyncStatusUpdate, pu)
}

func (eco *Eco) sendServiceStatus(su *ServiceStatus) {
	eco.state.Services[su.Service] = su
	eco.sendFeedMessage("", MsgTypeServiceStatus, su)
}

func (eco *Eco) sendFeedMessage(k string, msgType FeedMessageType, thing interface{}) {
	b, err := encode.GobEncode(thing)
	if err != nil {
		log.Errorf("Error encoding %s update: %v", msgType, err)
		return
	}
	msg := &FeedMessage{
		Type:     msgType,
		Contents: b,
	}
	if k != "" {
		eco.syncCache[k] = msg
	}
	for ch := range eco.syncChans {
		select {
		case ch <- msg:
		default:
			log.Warnf("Skipping %s update for blocking channel: %v", msgType, msg)
		}
	}
}

type releaseAssets struct {
	decred, decrediton, dexc *releaseAsset
	manifests                []*releaseAsset
	version                  string
}

type releaseAsset struct {
	*githubAsset
	path string
}

func (eco *Eco) initEco(conn net.Conn, req *initRequest) {
	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()

	prog := newProgressReporter(conn, "eco")

	if len(req.PW) == 0 && !walletFileExists() {
		prog.fail("Password required to initialize wallet", nil)
		return
	}

	// If we already have a version number and a password, we won't
	// re-initialize.
	if eco.state.Eco.Version != "" {
		b, _ := eco.db.Fetch(crypterKey)
		if len(b) > 0 {
			prog.fail("Eco is already initialized", nil)
			return
		}
	}

	prog.report(0.05, "Checking for updates")
	releases, err := fetchReleases()
	if err != nil {
		prog.fail("Error fetching releases", err)
		return
	}
	// For now, just get the most recent release, regardless of whether it is
	// pre-release. Eventually, we'll want to initially seed with latest stable
	// release, then offer pre-releases as a user preference.
	if len(releases) == 0 {
		prog.fail("No releases fetched", nil)
		return
	}
	release := releases[0]
	assets, err := parseAssets(release)
	if err != nil {
		prog.fail("Failed to parse assets", err)
		return
	}

	versionDir := filepath.Join(EcoDir, release.Name)

	skipDownload := false
	if skipDownload {
		log.Critical("Don't forget to remove skipDownload := true")
	} else { // Need a way to disable re-download during testing here.
		// All assets were found. Download and unpack them to a temporary directory.
		tmpDir, err := ioutil.TempDir("", "")
		if err != nil {
			prog.fail("Failed to create temporary directory", err)
			return
		}
		defer os.RemoveAll(tmpDir)

		// Fetch and parse the manifests.
		prog.report(0.1, "Downloading hash manifests")
		hashes := make(map[string][]byte)

		parseParts := func(line string) []string {
			parts := make([]string, 0, 2)
			for _, part := range strings.Split(line, " ") {
				if part == "" {
					continue
				}
				parts = append(parts, part)
			}
			return parts
		}

		log.Infof("Retrieving %d manifest files", len(assets.manifests))
		for _, m := range assets.manifests {
			log.Infof("Downloading %s", m.Name)
			m.path, err = fetchAsset(eco.outerCtx, tmpDir, m.URL, m.Name)
			if err != nil {
				prog.fail("Failed to fetch manifest", err)
				return
			}
			manifestFile, err := os.Open(m.path)
			if err != nil {
				prog.fail("Error opening manifest file", err)
				return
			}
			defer manifestFile.Close()

			scanner := bufio.NewScanner(manifestFile)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				parts := parseParts(line)
				if len(parts) != 2 {
					err := fmt.Errorf("Manifest line parse error. Expected 2 parts, got %d for %q: %q", len(parts), line, parts)
					prog.fail("Manifest parse error", err)
					return
				}
				b, err := hex.DecodeString(parts[0])
				if err != nil {
					prog.fail("Hex decode error", err)
					return
				}
				if len(b) != sha256.Size {
					err := fmt.Errorf("Invalid manifest hash length. Wanted %d, got %d", sha256.Size, len(b))
					prog.fail("Invalid manifest length", err)
					return
				}
				hashes[parts[1]] = b
			}

			if err := scanner.Err(); err != nil {
				prog.fail("Error reading manifest: %w", err)
				return
			}
		}

		// Fetch, unpack, and move all resources.
		err = moveResources(eco.outerCtx, tmpDir, assets, hashes, prog.subReporter(0.12, 0.80))
		if err != nil {
			prog.fail("Error moving assets", err)
			return
		}

		// Make sure we have a chromium browser for DEX. If we don't, but we know
		// where to get one, download it. If we don't know where to get one, we'll
		// have to deal with it at the UI level. E.g. a message saying "Open DEX in
		// your browser at ..."
		_, _, found, err := chromium(eco.outerCtx)
		if err != nil {
			prog.fail("Error searching for Chromium", err)
			return
		}
		if !found {
			// If we have a file to download, do it.
			err := downloadChromium(eco.outerCtx, tmpDir, versionDir, prog.subReporter(0.80, 0.85))
			if err != nil {
				log.Errorf("Error downloading Chromium: %v", err)
			}
		}

		// Update complete, store password and new eco state.
		crypter := encrypt.NewCrypter(req.PW)
		err = eco.db.Store(crypterKey, crypter.Serialize())
		if err != nil {
			err := fmt.Errorf("Upgraded to version %s, but failed to save encryption key to the DB: %w", release.Name, err)
			prog.fail("DB error storing encryption key", err)
			return
		}
	}

	// Need to cache the password until we can initialize DEX.
	pwc, err := newPWCache(req.PW)
	if err != nil {
		prog.fail("Encryption error", err)
		return
	}
	err = eco.db.EncodeStore(dexInputKey, pwc)
	if err != nil {
		prog.fail("Error storing dex input", err)
		return
	}

	if !walletFileExists() {
		prog.report(0.85, "Initializing dcrwallet")
		createWallet := func() bool {
			// Write the user's password to a file.
			passFile, err := ioutil.TempFile("", "")
			if err != nil {
				prog.fail("Error initializing wallet pass file", err)
				return false
			}
			// Delete the file asap.
			defer os.Remove(passFile.Name())
			passFile.Write([]byte(fmt.Sprintf("pass=%s\n", string(req.PW))))

			// Create a seed, and save it encrypted with the user's wallet
			// password until the user authorizes deletion.
			seed := encode.RandomBytes(32)
			crypter := encrypt.NewCrypter(req.PW)
			encSeed, err := crypter.Encrypt(seed)
			if err != nil {
				prog.fail("Error encrypting wallet seed", err)
				return false
			}
			err = eco.db.Store(walletSeedKey, encSeed)
			if err != nil {
				prog.fail("Error storing wallet seed", err)
				return false
			}

			exe := filepath.Join(versionDir, decred, dcrWalletExeName)

			err = nil
			eco.runContext(time.Second*5, func(ctx context.Context) {
				svcExe := newExe(eco.outerCtx, exe,
					fmt.Sprintf("--appdata=\"%s\"", dcrwalletAppDir),
					"--create",
					fmt.Sprintf("--configfile=\"%s\"", passFile.Name()),
				)

				var stdin io.WriteCloser
				stdin, err = svcExe.cmd.StdinPipe()
				if err != nil {
					prog.fail("Error writing wallet answers", err)
					return
				}
				go func() {
					defer stdin.Close()
					io.WriteString(stdin, fmt.Sprintf("y\nn\ny\n%x\n", seed)) // = y use file pass, n second pass for pubkey, y use seed, seed
				}()

				err = svcExe.Run()
				if err != nil {
					log.Errorf("Error creating wallet: err = %v, output = %s", err)
					prog.fail("Error creating wallet", err)
				}
			})
			if err != nil {
				return false
			}

			// Here's the problem. dcrwallet requires the password the first
			// time it is started. But we don't start dcrwallet until dcrd is
			// synced, so storing the password in memory until startup is not
			// good enough, since the user could kill Eco before sync is
			// complete, and we wouldn't have it during the next startup.
			// So, we'll store the password in the database until the wallet
			// is started and a sync has begun.
			pwc, err := newPWCache(req.PW)
			if err != nil {
				prog.fail("Encryption error", err)
				return false
			}
			err = eco.db.EncodeStore(extraInputKey, pwc)
			if err != nil {
				prog.fail("Error storing extra input", err)
				return false
			}

			return true
		}
		if !createWallet() {
			return
		}
	}

	eco.state.Eco.WalletExists = true // Can't get here without a wallet.
	eco.state.Eco.Version = release.Name
	eco.state.Eco.SyncMode = req.SyncMode
	err = eco.saveEcoState()
	if err != nil {
		err := fmt.Errorf("Upgraded to version %s, but failed to save new state to the DB: %w", release.Name, err)
		prog.fail("DB error storing eco state", err)
		return
	}

	// The client should close the connection up on receiving progress = 1.0.
	prog.report(1.0, "Upgrade complete")
	eco.sendServiceStatus(&ServiceStatus{Service: decrediton})
	go eco.start()
}

func (eco *Eco) saveEcoState() error {
	fmt.Println("--saveEcoState", dirtyEncode(eco.state))
	return eco.db.EncodeStore(ecoStateKey, eco.state.Eco)
}

func (eco *Eco) dcrdProcess() (*serviceExe, *rpcclient.Client) {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	return eco.dcrd.exe, eco.dcrd.client
}

func (eco *Eco) dcrWalletProcess() (*serviceExe, *walletclient.Client) {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	return eco.dcrwallet.exe, eco.dcrwallet.client
}

func (eco *Eco) runContext(dur time.Duration, f func(context.Context)) {
	callCtx, cancel := context.WithTimeout(eco.outerCtx, dur)
	defer cancel()
	f(callCtx)
}

func (eco *Eco) stopDCRD() error {
	if atomic.LoadUint32(&dcrdRunning) == 0 {
		return fmt.Errorf("Cannot stop dcrd. Not running")
	}
	svcExe, cl := eco.dcrdProcess()
	if cl == nil {
		return fmt.Errorf("Cannot stop dcrd. No client found")
	}
	if svcExe == nil {
		return fmt.Errorf("No serviceExe for dcrd")
	}
	_, err := cl.RawRequest(eco.innerCtx, "stop", nil)
	if err != nil {
		log.Errorf("dcrd RawRequest(stop) error: %v", err)
	}
	select {
	case <-svcExe.Done():
	case <-time.After(time.Second * 60):
		svcExe.cmd.Process.Kill()
		return fmt.Errorf("Timed out waiting for dcrd to shutdown. Killing the process")
	}
	return nil
}

func (eco *Eco) stopDCRWallet() error {
	if atomic.LoadUint32(&dcrWalletRunning) == 0 {
		return fmt.Errorf("Cannot stop dcrwallet. Not running")
	}
	svcExe, cl := eco.dcrWalletProcess()
	if cl == nil {
		return fmt.Errorf("Cannot stop dcrwallet. No client found")
	}
	if svcExe == nil {
		return fmt.Errorf("No serviceExe for dcrwallet")
	}
	err := cl.Call(eco.innerCtx, "stop", nil)
	if err != nil {
		log.Errorf("error calling 'stop' method for dcrwallet")
	}
	select {
	case <-svcExe.Done():
	case <-time.After(time.Second * 60):
		svcExe.cmd.Process.Kill()
		return fmt.Errorf("Timed out waiting for dcrd to shutdown. Killing the process")
	}
	return nil
}

func (eco *Eco) runDCRD() error {
	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()
	userSettings := &eco.dcrd.UserSettings

	if !atomic.CompareAndSwapUint32(&dcrdRunning, 0, 1) {
		return fmt.Errorf("dcrd already running")
	}

	eco.sendServiceStatus(&ServiceStatus{
		Service: dcrd,
		On:      true,
	})

	args := []string{
		fmt.Sprintf("--appdata=\"%s\"", dcrdAppDir),
		fmt.Sprintf("--debuglevel=%s", userSettings.DebugLevel),
		fmt.Sprintf("--rpclisten=%s", dcrdRPCListen),
		fmt.Sprintf("--rpcuser=%s", eco.dcrd.RPCUser),
		fmt.Sprintf("--rpcpass=%s", eco.dcrd.RPCPass),
		fmt.Sprintf("--listen=%s", dcrdListen),
	}

	go func() {
		defer atomic.StoreUint32(&dcrdRunning, 0)
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: dcrd,
			On:      false,
		})
		for {
			exe := filepath.Join(EcoDir, eco.state.Eco.Version, decred, dcrdExeName)

			svcExe := newExe(eco.innerCtx, exe, args...)
			eco.dcrd.exe = svcExe
			svcExe.Run()
			select {
			case <-time.After(time.Second * 5):
			case <-eco.outerCtx.Done():
				return
			}
		}

	}()

	go func() {
		var connectAttempts int

		// First, keep trying to get a client until successful. On initial
		// startup, this may fail until the TLS keypair is generated, which
		// is probably only once.
		var cl *rpcclient.Client
		for {
			var err error
			cl, err = eco.dcrdClient()
			if err == nil {
				break
			}
			connectAttempts++
			if connectAttempts >= 5 && connectAttempts%5 == 0 {
				log.Errorf("Error getting dcrd RPC client: %v", err)
			}
			select {
			case <-time.After(time.Second * 5):
			case <-eco.outerCtx.Done():
				return
			}
		}
		eco.stateMtx.Lock()
		eco.dcrd.client = cl
		eco.stateMtx.Unlock()

		defer func() {
			eco.stateMtx.Lock()
			eco.dcrd.client = nil
			eco.stateMtx.Unlock()
		}()

		getInfo := func() (bci *chainjson.GetBlockChainInfoResult) {
			var err error
			eco.runContext(time.Second, func(ctx context.Context) {
				bci, err = cl.GetBlockChainInfo(ctx)
			})
			if err != nil {
				log.Debugf("GetBlockChainInfo error: %v", err)
			}
			return bci
		}

		var bcInfo *chainjson.GetBlockChainInfoResult
		for {
			if eco.outerCtx.Err() != nil {
				return
			}
			if bcInfo = getInfo(); bcInfo == nil {
				select {
				case <-time.After(time.Second):
					continue
				case <-eco.outerCtx.Done():
					return
				}
			}
			break
		}
		startHeight := bcInfo.Blocks
		syncing := bcInfo.InitialBlockDownload || bcInfo.SyncHeight-startHeight > 1

		sendSyncUpdate := func() (synced bool) {
			if bcInfo = getInfo(); bcInfo == nil {
				return
			}
			h := bcInfo.SyncHeight
			if bcInfo.Headers > h {
				h = bcInfo.Headers
			}
			toGo := h - bcInfo.Blocks
			syncing = bcInfo.InitialBlockDownload || toGo > 1
			if !syncing {
				eco.signalDCRDSynced()
				eco.sendSyncUpdate(&Progress{
					Service:  dcrd,
					Status:   "Fully synced",
					Progress: 1.0,
				})
				return true
			}
			progress := 1 - float32(toGo)/float32(h-startHeight)
			eco.sendSyncUpdate(&Progress{
				Service:  dcrd,
				Status:   fmt.Sprintf("Syncing blockchain at block %d", bcInfo.Blocks),
				Progress: progress,
			})
			return
		}

		sendSyncUpdate()

		delay := time.Second * 5
		for {
			timer := time.NewTimer(delay)
			delay = time.Second * 5
			select {
			case <-timer.C:
				if !sendSyncUpdate() {
					delay = time.Second * 30
				}
			case <-eco.innerCtx.Done():
				timer.Stop()
				return
			}
		}
	}()
	return nil
}

func (eco *Eco) signalDCRDSynced() {
	if atomic.CompareAndSwapUint32(&dcrdSyncedOnce, 0, 1) {
		close(eco.dcrdSynced)
	}
}

func (eco *Eco) signalDCRWalletRunning() {
	if atomic.CompareAndSwapUint32(&dcrwalletRunningOnce, 0, 1) {
		close(eco.dcrwalletReady)
	}
}

func (eco *Eco) runDCRWallet() error {
	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()

	if !atomic.CompareAndSwapUint32(&dcrWalletRunning, 0, 1) {
		return fmt.Errorf("dcrwallet already running")
	}

	eco.sendServiceStatus(&ServiceStatus{
		Service: dcrwallet,
		On:      true,
	})

	// We use the same rpc name and pass and debug level for dcrd and dcrwallet.
	userSettings := &eco.dcrd.UserSettings

	args := []string{
		fmt.Sprintf("--appdata=\"%s\"", dcrwalletAppDir),
		fmt.Sprintf("--debuglevel=%s", userSettings.DebugLevel),
		fmt.Sprintf("--rpclisten=%s", dcrWalletRPCListen),
		fmt.Sprintf("--username=%s", eco.dcrd.RPCUser),
		fmt.Sprintf("--password=%s", eco.dcrd.RPCPass),
		fmt.Sprintf("--rpcconnect=127.0.0.1%s", dcrdRPCListen),
		fmt.Sprintf("--rpccert=\"%s\"", dcrWalletRPCCert),
		fmt.Sprintf("--rpckey=\"%s\"", dcrWalletRPCKey),
		"--nogrpc",
		fmt.Sprintf("--cafile=\"%s\"", dcrdCertPath),
	}

	spvMode := eco.state.Eco.SyncMode == SyncModeSPV
	if spvMode {
		args = append(args, "--spv")
	}

	extraInput := new(pwCache)
	hasExtraInput, err := eco.db.FetchDecode(extraInputKey, extraInput)
	if err != nil {
		log.Errorf("DB error fetching extra input: %v", err)
		return fmt.Errorf("DB error: %w", err)
	}

	// A goroutine to actually run the wallet.
	go func() {
		defer atomic.StoreUint32(&dcrWalletRunning, 0)
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: dcrwallet,
			On:      false,
		})
		if !spvMode {
			select {
			case <-eco.dcrdSynced:
			case <-eco.outerCtx.Done():
				return
			}
		}

		var pwAdded bool
		for {
			var svcExe *serviceExe
			if hasExtraInput && !pwAdded {
				pw, err := extraInput.PW()
				if err != nil {
					log.Errorf("Error deserializing crypter: %v", err)
					return
				}
				// I have tried getting this into stdin, but something about the
				// way dcrwallet accepts input seems to be whacky. This is a
				// security issue. Should be sure to at least lock the wallet
				// below.
				args = append(args, fmt.Sprintf("--pass=\"%s\"", string(pw)))
				pwAdded = true
				// Clear the password from the exe.Cmd in a minute.
				go func() {
					select {
					case <-time.After(60 * time.Second):
						if svcExe != nil && svcExe.cmd != nil {
							svcExe.cmd.Args = nil
						}
					}
				}()
			}

			// We might not have a version until initialized, so we can't create the
			// command before here.
			exe := filepath.Join(EcoDir, eco.state.Eco.Version, decred, dcrWalletExeName)
			svcExe = newExe(eco.innerCtx, exe, args...)
			eco.dcrwallet.exe = svcExe
			svcExe.Run()
			select {
			case <-time.After(time.Second * 5):
			case <-eco.outerCtx.Done():
				return
			}
		}
	}()

	// A goroutine to establish a connection and set the client.
	go func() {
		var connectAttempts int

		if !spvMode {
			select {
			case <-eco.dcrdSynced:
			case <-eco.outerCtx.Done():
				return
			}
		}

		// First, keep trying to get a client until successful. On initial
		// startup, this may fail until the TLS keypair is generated, which
		// is probably only once.

		var wcl *walletclient.Client
		// dcrdClient
		for {
			var err error
			wcl, err = eco.dcrWalletClient()
			if err == nil {
				break
			}
			connectAttempts++
			if connectAttempts >= 5 && connectAttempts%5 == 0 {
				log.Errorf("Error getting dcrwallet RPC client: %v", err)
			}
			select {
			case <-time.After(time.Second * 5):
			case <-eco.outerCtx.Done():
				return
			}
		}
		eco.stateMtx.Lock()
		eco.dcrwallet.client = wcl
		eco.stateMtx.Unlock()

		defer func() {
			eco.stateMtx.Lock()
			eco.dcrwallet.client = nil
			eco.stateMtx.Unlock()
		}()

		eco.signalDCRWalletRunning()

		getWalletInfo := func() *wallettypes.InfoWalletResult {
			var err error
			var nfo *wallettypes.InfoWalletResult

			eco.runContext(time.Second, func(ctx context.Context) {
				nfo, err = wcl.GetInfo(ctx)
			})
			if err != nil {
				log.Infof("getWalletInfo error: %v", err)
				return nil
			}

			return nfo
		}

		var walletInfo *wallettypes.InfoWalletResult
		for {
			if eco.outerCtx.Err() != nil {
				return
			}
			// dcrwallet will keep requesting the password for initial sync
			// until they have retrieved at least one block. Don't continue
			// until dcrwallet confirms they have it.
			if walletInfo = getWalletInfo(); walletInfo == nil || walletInfo.Blocks == 0 {
				select {
				case <-time.After(time.Second):
					continue
				case <-eco.outerCtx.Done():
					return
				}
			}
			break
		}

		fmt.Println("--a.10")

		// Delete the extraInput from the database, since it may contain
		// a password.
		if hasExtraInput {
			eco.db.Store(extraInputKey, nil)
		}

		// I guess just run a loop to keep checking the connection for now.
		// Maybe should be checking the walletInfo.Blocks against dcrd's
		// reported tip height for progress, but not sure what to do in SPV
		// mode. I don't believe that dcrwallet offers any information via RPC
		// on wallet sync status.
		delay := time.Second * 5
		synced := false
		for {
			timer := time.NewTimer(delay)
			delay = time.Second * 5
			select {
			case <-timer.C:
				fmt.Println("--checking sync", synced)

				if walletInfo = getWalletInfo(); walletInfo == nil {
					if synced {
						synced = false
						eco.sendSyncUpdate(&Progress{Service: dcrwallet, Err: "dcrwallet disconnected"})
					}
					continue
				}
				delay = time.Second * 30

				eco.stateMtx.RLock()
				cl := eco.dcrd.client
				syncMode := eco.state.Eco.SyncMode
				eco.stateMtx.RUnlock()

				if syncMode != SyncModeFull || cl == nil {
					continue
				}
				var err error
				var bci *chainjson.GetBlockChainInfoResult
				eco.runContext(time.Second, func(ctx context.Context) {
					bci, err = cl.GetBlockChainInfo(ctx)
				})
				u := &Progress{Service: dcrwallet}
				if err != nil {
					u.Err = "Wallet sync error"
					log.Infof("GetBlockChainInfo error in wallet loop: %v", err)
				} else {
					h := bci.SyncHeight
					if bci.Headers > h {
						h = bci.Headers
					}

					if h > 0 {
						if int64(walletInfo.Blocks) >= h {
							u.Progress = 1
							synced = true

							fmt.Println("--dcrwallet synced")
						}
						u.Progress = float32(walletInfo.Blocks) / float32(h)
					}

					u.Status = "Syncing"
				}
				eco.sendSyncUpdate(u)

			case <-eco.outerCtx.Done():
				timer.Stop()
				return
			}
		}
	}()

	return nil
}

func encodeToJSONFile(fp string, thing interface{}) error {
	f, err := os.OpenFile(decreditonConfigPath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Error reading decrediton configuration file")
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(thing)
}

func (eco *Eco) runDecrediton() error {
	// Decrediton does not allow a custom appdata directory. Open an issue/PR?.
	// So if there is already a configuration file, we'll be loading their
	// existing preferences, but overriding a few with command line args. On the
	// other hand, if there is not a file, we should create one with some
	// compatible defaults, e.g. dark mode to match Eco GUI.
	if !fileExists(decreditonConfigPath) {
		err := encodeToJSONFile(decreditonConfigPath, defaultDecreditonConfig())
		if err != nil {
			return fmt.Errorf("Error writing Decrediton config file")
		}
	}

	if !atomic.CompareAndSwapUint32(&decreditonRunning, 0, 1) {
		return fmt.Errorf("Decrediton already running")
	}

	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()

	eco.sendServiceStatus(&ServiceStatus{
		Service: decrediton,
		On:      true,
	})

	args := []string{
		fmt.Sprintf("--advanced"),
		fmt.Sprintf("--rpcuser=%s", eco.dcrd.RPCUser),
		fmt.Sprintf("--rpcpass=%s", eco.dcrd.RPCPass),
		fmt.Sprintf("--rpccert=%s", dcrdCertPath),
		fmt.Sprintf("--rpcconnect=%s", "localhost"+dcrdRPCListen),
		fmt.Sprintf("--custombinpath=%s", filepath.Join(EcoDir, eco.state.Eco.Version, decred)),
	}

	if eco.state.Eco.SyncMode == SyncModeSPV {
		args = append(args, "--spv")
	}

	exe := filepath.Join(EcoDir, eco.state.Eco.Version, decrediton, decreditonExeName)

	svcExe := newExe(eco.innerCtx, exe, args...)

	go func() {
		defer atomic.StoreUint32(&decreditonRunning, 0)
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: decrediton,
			On:      false,
		})

		svcExe.Run()
	}()

	return nil
}

type dexNewWalletForm struct {
	AssetID uint32            `json:"assetID"`
	Config  map[string]string `json:"config"`
	Pass    encode.PassBytes  `json:"pass"`
	AppPW   encode.PassBytes  `json:"appPass"`
}

func (eco *Eco) runDEX() error {
	select {
	case <-eco.dcrwalletReady:
	case <-eco.outerCtx.Done():
		return fmt.Errorf("context canceled")
	}

	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()

	rpcUser, rpcPass := eco.dcrd.RPCUser, eco.dcrd.RPCPass

	dexInput := new(pwCache)
	initializing, err := eco.db.FetchDecode(dexInputKey, dexInput)
	if err != nil {
		return fmt.Errorf("Error loading dex input: %v", err)
	}

	// Allow initialization in SPV so that we can delete the cached credentials
	// from the DB.
	if !initializing && eco.state.Eco.SyncMode == SyncModeSPV {
		return fmt.Errorf("Cannot run DEX in SPV mode")
	}

	if !atomic.CompareAndSwapUint32(&dexRunning, 0, 1) {
		return fmt.Errorf("DEX already running")
	}

	args := []string{
		fmt.Sprintf("--appdata=\"%s\"", dexAppDir),
		fmt.Sprintf("--webaddr=%s", "localhost"+dexWebAddr),
	}

	exe := filepath.Join(EcoDir, eco.state.Eco.Version, dexc, dexcExeName)

	svcExe := newExe(eco.innerCtx, exe, args...)

	go func() {
		defer atomic.StoreUint32(&dexRunning, 0)
		svcExe.Run()
	}()

	initialize := func() error {
		// First, try to create a new wallet account.
		eco.stateMtx.RLock()
		cl := eco.dcrwallet.client
		eco.stateMtx.RUnlock()
		if cl == nil {
			return fmt.Errorf("Cannot initialize DEX: No dcrwallet rpc client found")
		}
		pwb, err := dexInput.PW()
		if err != nil {
			return fmt.Errorf("Error getting credentials from cache: %w", err)
		}
		pw := encode.PassBytes(pwb)
		defer pw.Clear()

		accts, err := cl.ListAccounts(eco.outerCtx)
		if err != nil {
			return fmt.Errorf("Error listing accounts: %w", err)
		}
		// If no DEX account is found, create the account.
		if _, found := accts[dexAcctName]; !found {
			err = cl.WalletPassphrase(eco.outerCtx, string(pw), int64(time.Duration(math.MaxInt64)/time.Second))
			if err != nil {
				return fmt.Errorf("Error unlocking wallet: %w", err)
			}
			log.Infof("Creating new 'dex' account")
			err = cl.CreateNewAccount(eco.outerCtx, dexAcctName)
			if err != nil {
				return fmt.Errorf("Error creating new account: %w", err)
			}
		}

		// Get the core.User.
		user := &struct {
			Initialized bool `json:"inited"`
			Authed      bool `json:"authed"`
			Assets      map[uint32]struct {
				Symbol string    `json:"symbol"`
				Wallet *struct{} `json:"wallet"`
			} `json:"assets"`
		}{}
		resp, err := http.Get("http://localhost" + dexWebAddr + "/api/user")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(user)
		if err != nil {
			return fmt.Errorf("JSON decode error: %w", err)
		}

		pwMsg := &struct {
			Pass encode.PassBytes `json:"pass"`
		}{
			Pass: pw,
		}

		request := dexCaller()

		if !user.Initialized {
			_, err := request(eco.outerCtx, "init", pwMsg)
			if err != nil {
				return err
			}
		} else {
			// Login
			_, err := request(eco.outerCtx, "login", pwMsg)
			if err != nil {
				return err
			}
		}

		// Create wallet
		if user.Assets[42].Wallet == nil {
			newWalletForm := &dexNewWalletForm{
				AssetID: 42,
				Config: map[string]string{
					"account":   dexAcctName,
					"username":  rpcUser,
					"password":  rpcPass,
					"rpclisten": "127.0.0.1" + dcrWalletRPCListen,
					"rpccert":   dcrWalletRPCCert,
				},
				Pass:  pw,
				AppPW: pw,
			}
			_, err = request(eco.outerCtx, "newwallet", newWalletForm)
			if err != nil && !strings.Contains(err.Error(), "already initialized") {
				return err
			}
		}
		return nil
	}

	// If we need to initialize, run a second goroutine to attempt initial
	// setup.
	if initializing {
		go func() {
			for {
				err := initialize()
				if err == nil {
					eco.db.Store(dexInputKey, nil)
					// Send a ServiceStatus to trigger DEX service availability.
					eco.sendServiceStatus(&ServiceStatus{
						Service: dexc,
						On:      false,
					})

					// Kill dexc. The service has not even been available until
					// the serviceExe.Run goroutine exits, so the user does
					// not expect it to be running. They can now manually
					// start dexc.

					// Need a cleaner way to do this though.
					svcExe.cancel()
					break
				}
				log.Error(err)
				select {
				case <-time.After(time.Second * 5):
				case <-eco.outerCtx.Done():
					return
				}
			}

		}()
	}

	return nil
}

func (eco *Eco) openDEXWindow() error {
	if atomic.LoadUint32(&dexRunning) == 0 {
		err := eco.runDEX()
		if err != nil {
			return fmt.Errorf("Error starting DEX: %w", err)
		}
	}

	chromiumPath, args, found, err := chromium(eco.outerCtx)
	if err != nil {
		return fmt.Errorf("Error searching for browser: %v", err)
	}
	if !found {
		return fmt.Errorf("Failed to locate chromium-based browser")
	}

	// Or should we allow opening multiple windows?
	if !atomic.CompareAndSwapUint32(&dexWindowOpen, 0, 1) {
		return fmt.Errorf("DEX window already open")
	}

	defer eco.sendServiceStatus(&ServiceStatus{
		Service: dexc,
		On:      true,
	})

	// Keep pinging DEX until a connection is made, before opening the window.
	go func() {
		defer atomic.StoreUint32(&dexWindowOpen, 0)
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: dexc,
			On:      false,
		})

		var connectAttempts int
		for {
			connectAttempts++
			if eco.outerCtx.Err() != nil {
				return
			}
			_, err := http.Get("http://localhost" + dexWebAddr + "/api/user")
			if err == nil {
				break
			}
			if connectAttempts > 30 {
				log.Errorf("Failed to open DEX window")
				return
			}
			select {
			case <-time.After(time.Second):
			case <-eco.outerCtx.Done():
				return
			}
		}

		newExe(eco.outerCtx, chromiumPath, args...).Run()
	}()
	return nil
}
func extractMethod(cmd string) string {
	for _, token := range strings.Split(cmd, " ") {
		if token != "" {
			return token
		}
	}
	return ""
}

// based on https://stackoverflow.com/a/47489846/1124661
func tokenizeCmd(cmd string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(cmd))
	r.Comma = ' ' // space
	return r.Read()
}

func (eco *Eco) dcrctl(req *dcrCtlRequest) (*dcrCtlResponse, error) {
	tokens, err := tokenizeCmd(req.Cmd)
	if err != nil {
		return nil, fmt.Errorf("error parsing command: %v", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no command")
	}
	method := tokens[0]
	switch method {
	case "stop": // "walletlock", "walletpassphrase"
		return nil, fmt.Errorf("method not allowed by Eco")
	}
	// Try dcrwallet first, then dcrd. It would be nice to use the
	// Call/RawRequest methods, but we have no idea how to type the args.
	eco.stateMtx.RLock()
	version := eco.state.Eco.Version
	rpcUser, rpcPass := eco.dcrd.RPCUser, eco.dcrd.RPCPass
	eco.stateMtx.RUnlock()
	if version == "" {
		return nil, fmt.Errorf("eco not initialized")
	}

	preArgs := []string{
		fmt.Sprintf("--rpcuser=%s", rpcUser),
		fmt.Sprintf("--rpcpass=%s", rpcPass),
	}

	exe := filepath.Join(EcoDir, version, decred, dcrctl)
	var op []byte
	eco.runContext(time.Second*60, func(ctx context.Context) {
		args := preArgs
		args = append(args, fmt.Sprintf("--rpcserver=127.0.0.1%s", dcrWalletRPCListen))
		args = append(args, fmt.Sprintf("--rpccert=\"%s\"", dcrWalletRPCCert))
		args = append(args, "--wallet")
		args = append(args, tokens...)
		cmd := exec.CommandContext(ctx, exe, args...)
		cmd.Dir = filepath.Dir(exe)
		op, err = cmd.Output()
	})
	if err == nil {
		return &dcrCtlResponse{Body: string(op)}, nil
	}

	// Try dcrd then.
	eco.runContext(time.Second*60, func(ctx context.Context) {
		args := preArgs
		args = append(args, fmt.Sprintf("--rpcserver=127.0.0.1%s", dcrdRPCListen))
		args = append(args, fmt.Sprintf("--rpccert=\"%s\"", dcrdCertPath))
		args = append(args, req.Cmd)
		cmd := exec.CommandContext(ctx, exe, args...)
		cmd.Dir = filepath.Dir(exe)
		op, err = cmd.CombinedOutput()
	})
	if err != nil {
		return &dcrCtlResponse{Err: fmt.Sprintf("%v: %s", err, string(op))}, nil
	}
	return &dcrCtlResponse{Body: string(op)}, nil

}

func fetchAsset(ctx context.Context, dir string, url, name string) (string, error) {
	tgt := filepath.Join(dir, name)
	payload, err := os.OpenFile(tgt, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("Error creating file: %w", err)
	}
	defer payload.Close()

	// From github API docs...
	// > To download the asset's binary content, set the "Accept" header of the
	//   request to "application/octet-stream"
	client := &http.Client{}
	log.Infof("Fetching %q to %q", url, tgt)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("Error preparing request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Request error for %q %w", url, err)
	}

	defer resp.Body.Close()

	_, err = io.Copy(payload, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error saving archive to file: %v", err)
	}
	return payload.Name(), nil
}

func parseAssets(release *githubRelease) (*releaseAssets, error) {
	// Find all the manifest files
	assets := &releaseAssets{version: release.Name}
	for _, asset := range release.Assets {
		if manifestPattern.Match([]byte(asset.Name)) {
			assets.manifests = append(assets.manifests, &releaseAsset{githubAsset: asset})
		}
	}

	findAsset := func(re *regexp.Regexp) *releaseAsset {
		for _, asset := range release.Assets {
			k := []byte(asset.Name)
			matches := re.FindSubmatch(k)
			if len(matches) > 0 {
				return &releaseAsset{githubAsset: asset}
			}
		}
		return nil
	}

	if assets.decred = findAsset(decredPattern); assets.decred == nil {
		return nil, fmt.Errorf("no decred archive found in release %s", release.Name)
	}
	if assets.decrediton = findAsset(decreditonPattern); assets.decrediton == nil {
		return nil, fmt.Errorf("no decrediton archive found in release %s", release.Name)
	}
	if assets.dexc = findAsset(dexcPattern); assets.dexc == nil {
		return nil, fmt.Errorf("no dexc archive found in release %s", release.Name)
	}
	return assets, nil
}

func (eco *Eco) filepath(subpaths ...string) string {
	return filepath.Join(append([]string{AppDir}, subpaths...)...)
}

type progressReporter struct {
	report func(p float32, s string, a ...interface{})
	fail   func(s string, err error)
}

func newProgressReporter(conn net.Conn, svc string) *progressReporter {
	var reportErr error
	return &progressReporter{
		report: func(p float32, s string, a ...interface{}) {
			if reportErr != nil {
				return
			}
			reportErr = sendProgress(conn, svc, fmt.Sprintf(s, a...), "", p)
			if reportErr != nil {
				log.Errorf("Error reporting progress: %v", reportErr)
			}
		},
		fail: func(s string, err error) {
			if err != nil {
				log.Errorf("%s: %v", s, err)
			} else {
				log.Error(s)
			}
			sendProgress(conn, svc, "", s, 0)
		},
	}
}

func (r *progressReporter) subReporter(start, end float32) *progressReporter {
	vRange := end - start
	return &progressReporter{
		report: func(p float32, s string, a ...interface{}) {
			r.report(start+(vRange*p), s, a...)
		},
		fail: r.fail,
	}
}

func checkFileHash(archivePath string, checkHash []byte) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("Error opening archive %q for hashing: %w", archivePath, err)
	}
	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	f.Close()
	if err != nil {
		return fmt.Errorf("Error hashing archive %q: %w", archivePath, err)
	}
	h := hasher.Sum(nil)
	if !bytes.Equal(h, checkHash) {
		return fmt.Errorf("File hash mismatch for %q. Expected %x, got %x", archivePath, checkHash, h)
	}
	return nil
}

func State(ctx context.Context) (state *MetaState, err error) {
	err = serviceStatus(ctx, "eco", &state)
	return
}

func Init(ctx context.Context, pw string, syncMode SyncMode) (<-chan *Progress, error) {
	ch := make(chan *Progress, 1)
	send := func(u *Progress) bool {
		select {
		case ch <- u:
			return true
		default:
		}
		return false
	}

	go func() {
		err := genericFeed(ctx, routeInit, &initRequest{
			SyncMode: syncMode,
			PW:       []byte(pw),
		}, func(ok bool, b []byte) bool {
			if !ok {
				send(&Progress{
					Service: routeInit,
					Err:     fmt.Sprintf("%s channel closed", routeInit),
				})
				return false
			}
			u := new(Progress)
			err := encode.GobDecode(b, u)
			if err != nil {
				log.Errorf("Error decoding %s progress update: %v", routeInit, err)
				send(&Progress{
					Service: routeInit,
					Err:     fmt.Sprintf("%s update decode error", routeInit),
				})
				return false
			}
			if !send(u) {
				return false
			}
			return true
		})
		if err != nil {
			log.Errorf("Init feed error: %v", err)
		}
	}()

	return ch, nil
}

type EcoFeeders struct {
	SyncStatus    func(*Progress)
	ServiceStatus func(*ServiceStatus)
}

type FeedMessageType uint16

const (
	MsgTypeInvalid FeedMessageType = iota
	MsgTypeSyncStatusUpdate
	MsgTypeServiceStatus
)

var feedMsgStrings = []string{
	"MsgTypeInvalid",
	"MsgTypeSyncStatusUpdate",
	"MsgTypeServiceStatus",
}

func (i FeedMessageType) String() string {
	if int(i) < len(feedMsgStrings) {
		return feedMsgStrings[i]
	}
	return "unknown feed message type"
}

type FeedMessage struct {
	Type     FeedMessageType
	Contents []byte
}

func newFeedMessage(typeID FeedMessageType, contents interface{}) (*FeedMessage, error) {
	b, err := encode.GobEncode(contents)
	if err != nil {
		return nil, err
	}
	return &FeedMessage{
		Type:     typeID,
		Contents: b,
	}, nil
}

func Feed(ctx context.Context, feeders *EcoFeeders) {
	log.Infof("Starting Eco Feed")
	defer log.Infof("Eco Feed closing")
	for {
		err := genericFeed(ctx, routeSync, []struct{}{}, func(ok bool, b []byte) bool {
			if !ok {
				log.Errorf("Sync feed closed")
				return false
			}
			msg := new(FeedMessage)
			err := encode.GobDecode(b, msg)
			if err != nil {
				log.Errorf("Error decoding sync feed message: %v", err)
				return false
			}
			switch msg.Type {
			case MsgTypeSyncStatusUpdate:
				u := new(Progress)
				err := encode.GobDecode(msg.Contents, u)
				if err != nil {
					log.Errorf("Error decoding Progress: %v", err)
					return false
				}
				feeders.SyncStatus(u)
			case MsgTypeServiceStatus:
				u := new(ServiceStatus)
				err := encode.GobDecode(msg.Contents, u)
				if err != nil {
					log.Errorf("Error decoding ServiceStatusUpdate: %v", err)
					return false
				}
				feeders.ServiceStatus(u)
			}
			return true
		})
		if err != nil {
			log.Errorf("Sync feed exited with error: %v", err)
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func StartDecrediton(ctx context.Context) {
	request(ctx, routeStartDecrediton, struct{}{}, nil)
}

func StartDEX(ctx context.Context) {
	request(ctx, routeStartDEX, struct{}{}, nil)
}

type dcrCtlRequest struct {
	Cmd string
}

type dcrCtlResponse struct {
	Err  string
	Body string
}

func DCRCtl(ctx context.Context, cmd string) (string, error) {
	resp := new(dcrCtlResponse)
	err := request(ctx, routeDCRCtl, &dcrCtlRequest{
		Cmd: cmd,
	}, resp)
	if err != nil {
		return "", err
	}
	if resp.Err != "" {
		return "", fmt.Errorf(resp.Err)
	}
	return resp.Body, nil
}

func walletFileExists() bool {
	return fileExists(filepath.Join(dcrwalletAppDir, "mainnet", "wallet.db"))
}

func genericFeed(ctx context.Context, route string, req interface{}, f func(bool, []byte) bool) error {
	cl, err := NewClient()
	if err != nil {
		return err
	}
	bChan, err := cl.subscribe(ctx, route, req)
	if err != nil {
		return err
	}

	for {
		select {
		case b, ok := <-bChan:
			if !f(ok, b) {
				return fmt.Errorf("function failure")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func svcKey(svc string) string {
	return "service#" + svc
}

func syncKey(svc string) string {
	return "sync#" + svc
}

func statusKey(svc string) string {
	return "status#" + svc
}

func dexCaller() func(context.Context, string, interface{}) ([]byte, error) {
	cj, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		panic("could not create cookie jar")
	}
	cl := http.Client{
		Jar: cj,
	}

	return func(ctx context.Context, route string, thing interface{}) ([]byte, error) {
		var b []byte
		if thing != nil {
			var err error
			b, err = json.Marshal(thing)
			if err != nil {
				return nil, fmt.Errorf("JSON encode error: %v", err)
			}
		}
		timedCtx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		req, err := http.NewRequestWithContext(timedCtx, "POST", "http://localhost"+dexWebAddr+"/api/"+route, bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("Error creating request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := cl.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s error: %v", route, err)
		}

		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Response body read error: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Request error: code = %d, msg = %s", resp.StatusCode, string(b))
		}

		return b, nil
	}
}

func dirtyEncode(thing interface{}) string {
	b, _ := json.Marshal(thing)
	return string(b)
}
