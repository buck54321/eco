package eco

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
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
)

const (
	UnixSocketFilename = "decred.sock"
	TCPSocketHost      = ":45219"
	ListenerFilename   = "addr.txt"
	dbFilename         = "eco.db"

	crypterKey    = "crypter"
	walletSeedKey = "walletSeed"
	extraInputKey = "extraInput"
	ecoStateKey   = "ecoState"
)

var (
	KeyPath  = filepath.Join(AppDir, "decred-eco.key")
	CertPath = filepath.Join(AppDir, "decred-eco.cert")

	dcrdRunning, dcrdSyncedOnce, dcrWalletRunning,
	decreditonRunning uint32

	osUser, _ = user.Current()
)

type Config struct {
	ServerAddress *NetAddr // [network, address]
	ExeExt        string   // with leading full stop
}

type NetAddr struct {
	Net  string
	Addr string
}

type Eco struct {
	cfg        *Config
	db         *db.DB
	innerCtx   context.Context
	outerCtx   context.Context
	dcrdSynced chan struct{}

	syncMtx   sync.Mutex
	syncCache map[string]*FeedMessage
	syncChans map[chan *FeedMessage]struct{}

	stateMtx   sync.RWMutex
	state      MetaState
	versionDir string
	dcrd       *DCRD
	dcrwallet  *DCRWallet
}

func Run(outerCtx context.Context, cfg *Config) {
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

	// Parse the server address
	if cfg.ServerAddress == nil {
		switch runtime.GOOS {
		case "linux":
			cfg.ServerAddress = &NetAddr{"unix", filepath.Join(AppDir, UnixSocketFilename)}
		default:
			cfg.ServerAddress = &NetAddr{"tcp4", TCPSocketHost}
		}
	}

	// Write the server address to file
	listenerFilepath := filepath.Join(AppDir, ListenerFilename)
	err = ioutil.WriteFile(listenerFilepath, []byte(fmt.Sprintf("%s %s", cfg.ServerAddress.Net, cfg.ServerAddress.Addr)), 0644)
	if err != nil {
		log.Errorf("Error writing listener file: %w", err)
		return
	}

	// We need an inner Context that is delayed on cancellation to allow clean
	// shutdown of e.g. dcrd
	innerCtx, cancel := context.WithCancel(context.Background())

	eco := &Eco{
		cfg:      cfg,
		db:       dbb,
		innerCtx: innerCtx,
		outerCtx: outerCtx,
		state: MetaState{
			Eco:      *state,
			Services: make(map[string]*ServiceStatus),
		},
		versionDir: filepath.Join(programDirectory, state.Version),
		dcrd:       &DCRD{DCRDState: *dcrdState},
		dcrwallet:  &DCRWallet{DCRWalletState: *dcrWalletState},
		syncChans:  make(map[chan *FeedMessage]struct{}),
		dcrdSynced: make(chan struct{}),
		syncCache:  make(map[string]*FeedMessage),
	}

	go func() {
		<-outerCtx.Done()
		time.AfterFunc(time.Second*30, func() { cancel() })
		eco.stopDCRD() // Ignore any errors.
		cancel()
	}()

	if eco.state.Eco.SyncMode == SyncModeFull {
		err = eco.runDCRD()
		if err != nil {
			log.Errorf("dcrd startup error: %w", err)
		}
	}
	err = eco.runDCRWallet()
	if err != nil {
		log.Errorf("dcrwallet startup error: %w", err)
	}

	for {
		srv, err := NewServer(eco.cfg.ServerAddress, eco)
		if err == nil {
			srv.Run(outerCtx)
		} else {
			// If we didn't even create the server, something is desperately
			// wrong.
			log.Errorf("Error creating Eco server: %v", err)
			return
		}

		if outerCtx.Err() != nil {
			return
		}
		// We only get here with an unkown server error. Wait a second and loop
		// again.
		time.Sleep(time.Second)
	}
	<-innerCtx.Done()
}

func (eco *Eco) shutdown() {
	eco.stopDCRD() // ignore errors
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
	var reportErr error
	report := func(p float32, s string, a ...interface{}) {
		if reportErr != nil {
			return
		}
		reportErr = sendProgress(conn, "eco", fmt.Sprintf(s, a...), "", p)
		if reportErr != nil {
			log.Errorf("Error reporting progress: %v", reportErr)
		}
	}
	fail := func(s string, err error) {
		if err != nil {
			log.Errorf("%s: %v", s, err)
		} else {
			log.Error(s)
		}
		sendProgress(conn, "eco", "", s, 0)
	}

	if len(req.PW) == 0 && !walletFileExists() {
		fail("Password required to initialize wallet", nil)
		return
	}

	// If we already have a version number and a password, we won't
	// re-initialize.
	if eco.state.Eco.Version != "" {
		b, _ := eco.db.Fetch(crypterKey)
		if len(b) > 0 {
			fail("Eco is already initialized", nil)
			return
		}
	}

	report(0.05, "Checking for updates")
	releases, err := fetchReleases()
	if err != nil {
		fail("Error fetching releases", err)
		return
	}
	// For now, just get the most recent release, regardless of whether it is
	// pre-release. Eventually, we'll want to initially seed with latest stable
	// release, then offer pre-releases as a user preference.
	if len(releases) == 0 {
		fail("No releases fetched", nil)
		return
	}
	release := releases[0]
	assets, err := parseAssets(release)
	if err != nil {
		fail("Failed to parse assets", err)
		return
	}

	if !walletFileExists() {
		createWallet := func() bool {
			// Write the user's password to a file.
			passFile, err := ioutil.TempFile("", "")
			if err != nil {
				fail("Error initializing wallet pass file", err)
				return false
			}
			defer os.Remove(passFile.Name())
			passFile.Write([]byte(fmt.Sprintf("pass=%s\n", string(req.PW))))

			// Create a seed, and save it encrypted with the user's wallet
			// password until the user authorizes deletion.
			seed := encode.RandomBytes(32)
			crypter := encrypt.NewCrypter(req.PW)
			encSeed, err := crypter.Encrypt(seed)
			if err != nil {
				fail("Error encrypting wallet seed", err)
				return false
			}
			err = eco.db.Store(walletSeedKey, encSeed)
			if err != nil {
				fail("Error storing wallet seed", err)
				return false
			}

			exe := filepath.Join(programDirectory, release.Name, decred, dcrWalletExeName)
			cmd := exec.CommandContext(eco.outerCtx, exe, []string{
				fmt.Sprintf("--appdata=\"%s\"", dcrwalletAppDir),
				"--create",
				fmt.Sprintf("--configfile=\"%s\"", passFile.Name()),
			}...)
			cmd.Dir = filepath.Dir(exe)

			stdin, err := cmd.StdinPipe()
			if err != nil {
				fail("Error writing wallet answers", err)
				return false
			}
			go func() {
				defer stdin.Close()
				io.WriteString(stdin, fmt.Sprintf("y\nn\ny\n%x\n", seed)) // = y use file pass, n second pass for pubkey, y use seed, seed
			}()

			op, err := cmd.Output()
			if err != nil {
				log.Infof("Error creating wallet: err = %v, output = %s", err, string(op))
				fail("Error creating wallet", err)
				return false
			}

			// Here's the problem. dcrwallet requires the password the first
			// time it is started. But we don't start dcrwallet until dcrd is
			// synced, so storing the password in memory until startup is not
			// good enough, since the user could kill Eco before sync is
			// complete, and we wouldn't have it during the next startup.
			// So, we'll store the password in the database until the wallet
			// is started and a sync has begun.
			eco.db.Store(extraInputKey, req.PW)

			return true
		}
		if !createWallet() {
			return
		}
	}

	// All assets were found. Download and unpack them to a temporary directory.
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		fail("Failed to create temporary directory", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Fetch and parse the manifests.
	report(0.1, "Downloading the hash manifests")
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
		m.path, err = fetchAsset(eco.outerCtx, tmpDir, m)
		if err != nil {
			fail("Failed to fetch manifest", err)
			return
		}
		manifestFile, err := os.Open(m.path)
		if err != nil {
			fail("Error opening manifest file", err)
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
				fail("Manifest parse error", err)
				return
			}
			b, err := hex.DecodeString(parts[0])
			if err != nil {
				fail("Hex decode error", err)
				return
			}
			if len(b) != sha256.Size {
				err := fmt.Errorf("Invalid manifest hash length. Wanted %d, got %d", sha256.Size, len(b))
				fail("Invalid manifest length", err)
				return
			}
			hashes[parts[1]] = b
		}

		if err := scanner.Err(); err != nil {
			fail("Error reading manifest: %w", err)
			return
		}
	}

	// Fetch, unpack, and move all resources.
	err = moveResources(eco.outerCtx, tmpDir, assets, hashes, report)
	if err != nil {
		fail("Error moving assets", err)
		return
	}

	// // Update complete, store password and new eco state.
	// crypter := encrypt.NewCrypter(req.PW)
	// err = eco.db.Store(crypterKey, crypter.Serialize())
	// if err != nil {
	// 	err := fmt.Errorf("Upgraded to version %s, but failed to save encryption key to the DB: %w", release.Name, err)
	// 	fail("DB error storing encryption key", err)
	// 	return
	// }

	eco.stateMtx.Lock()
	eco.state.Eco.WalletExists = true // Can't get here without a wallet.
	eco.state.Eco.Version = release.Name
	eco.state.Eco.SyncMode = req.SyncMode
	err = eco.saveEcoState()
	eco.stateMtx.Unlock()
	if err != nil {
		err := fmt.Errorf("Upgraded to version %s, but failed to save new state to the DB: %w", release.Name, err)
		fail("DB error storing eco state", err)
		return
	}

	// Begin the dcrd sync.
	err = eco.runDCRD()
	if err != nil {
		err := fmt.Errorf("dcrd error: %w", err)
		fail("dcrd startup error", err)
		return
	}

	// The client should close the connection up on receiving progress = 1.0.
	report(1.0, "Upgrade complete")
}

func (eco *Eco) dcrdProcess() (*serviceExe, *rpcclient.Client) {
	eco.stateMtx.RLock()
	defer eco.stateMtx.RUnlock()
	return eco.dcrd.exe, eco.dcrd.client
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
	exe, cl := eco.dcrdProcess()
	if exe == nil {
		return fmt.Errorf("No dcrd process found")
	}
	if cl == nil {
		return fmt.Errorf("Cannot stop dcrd. No client found")
	}
	var err error
	eco.runContext(time.Second, func(ctx context.Context) {
		_, err = cl.RawRequest(ctx, "stop", nil)
	})
	if err != nil {
		return err
	}
	// Give it a few seconds to shut down on its own.
	select {
	case <-exe.Done():
	case <-time.After(time.Second * 60):
		exe.cmd.Process.Kill()
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
			exe := filepath.Join(programDirectory, eco.state.Eco.Version, decred, dcrdExeName)

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

		tryGetInfo := func() (bci *chainjson.GetBlockChainInfoResult) {
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
			if bcInfo = tryGetInfo(); bcInfo == nil {
				select {
				case <-time.After(time.Second):
					continue
				case <-eco.outerCtx.Done():
					return
				}
				continue
			}
			break
		}
		startHeight := bcInfo.Blocks
		syncing := bcInfo.InitialBlockDownload || bcInfo.SyncHeight-startHeight > 1
		delay := time.Second * 5
		if !syncing {
			eco.signalDCRDSynced()
			// Send a progress report for fully synced.
			eco.sendSyncUpdate(&Progress{
				Service:  dcrd,
				Status:   "Fully synced",
				Progress: 1.0,
			})
			delay = time.Second * 30
		}

		for {
			timer := time.NewTimer(delay)
			delay = time.Second * 5
			select {
			case <-timer.C:
				if bcInfo = tryGetInfo(); bcInfo == nil {
					timer.Stop()
					continue
				}
				toGo := bcInfo.SyncHeight - bcInfo.Blocks
				syncing = bcInfo.InitialBlockDownload || toGo > 1
				if !syncing {
					eco.signalDCRDSynced()
					eco.sendSyncUpdate(&Progress{
						Service:  dcrd,
						Status:   "Fully synced",
						Progress: 1.0,
					})
					delay = time.Second * 30
					continue
				}
				progress := 1 - float32(toGo)/float32(bcInfo.SyncHeight-startHeight)
				eco.sendSyncUpdate(&Progress{
					Service:  dcrd,
					Status:   fmt.Sprintf("Syncing blockchain at block %d", bcInfo.Blocks),
					Progress: progress,
				})

			case <-eco.outerCtx.Done():
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
		fmt.Sprintf("--rpcconnect=%s", dcrdRPCListen),
		fmt.Sprintf("--rpccert=\"%s\"", dcrWalletRPCCert),
		fmt.Sprintf("--rpckey=\"%s\"", dcrWalletRPCKey),
		fmt.Sprintf("--cafile=\"%s\"", dcrdCertPath),
	}

	spvMode := eco.state.Eco.SyncMode == SyncModeSPV
	if spvMode {
		args = append(args, "--spv")
	}

	exeRunning := make(chan struct{})
	var svcExe *serviceExe

	// A goroutine to actually run the wallet.
	clearInput := false
	go func() {
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: dcrwallet,
			On:      false,
		})
		if !spvMode {
			defer atomic.StoreUint32(&dcrWalletRunning, 0)
			select {
			case <-eco.dcrdSynced:
			case <-eco.outerCtx.Done():
				return
			}
		}
		extraInput, err := eco.db.Fetch(extraInputKey)
		if err != nil {
			log.Errorf("DB error fetching extra input: %v", err)
			return
		}
		// We might not have a version until initialized, so we can't create the
		// command before here.
		exe := filepath.Join(programDirectory, eco.state.Eco.Version, decred, dcrWalletExeName)
		svcExe = newExe(eco.innerCtx, exe, args...)
		eco.dcrwallet.exe = svcExe

		if len(extraInput) > 0 {
			clearInput = true
			args := append(extraInput, byte('\n'))
			stdin, err := svcExe.cmd.StdinPipe()
			if err != nil {
				log.Errorf("Error getting stdin for dcrwallet command: %v", err)
				return
			} else {
				go func() {
					_, err := stdin.Write(args)
					if err != nil {
						log.Errorf("Error writing to dcrwallet stdin: %v", err)
					}
				}()
			}
		}
		close(exeRunning)
		svcExe.Run()
	}()

	// A goroutine to establish a connection and set the client.
	go func() {
		var connectAttempts int

		select {
		case <-exeRunning:
		case <-eco.outerCtx.Done():
			return
		}

		// First, keep trying to get a client until successful. On initial
		// startup, this may fail until the TLS keypair is generated, which
		// is probably only once.

		var cl *walletclient.Client
		for {
			var err error
			cl, err = eco.dcrWalletClient()
			if err == nil {
				break
			}
			connectAttempts++
			if connectAttempts >= 5 && connectAttempts%5 == 0 {
				log.Errorf("Error getting dcrwallet RPC client: %v", err)
			}
			select {
			case <-time.After(time.Second * 5):
			case <-svcExe.ctx.Done():
				return
			}
		}
		eco.stateMtx.Lock()
		eco.dcrwallet.client = cl
		eco.stateMtx.Unlock()

		defer func() {
			eco.stateMtx.Lock()
			eco.dcrwallet.client = nil
			eco.stateMtx.Unlock()
		}()

		tryGetInfo := func() *wallettypes.InfoWalletResult {
			var err error
			var nfo *wallettypes.InfoWalletResult

			eco.runContext(time.Second, func(ctx context.Context) {
				nfo, err = cl.GetInfo(ctx)
			})
			if err != nil {
				log.Debugf("GetInfo error: %v", err)
			}
			return nfo
		}

		var walletInfo *wallettypes.InfoWalletResult
		for {
			if svcExe.ctx.Err() != nil {
				return
			}
			if walletInfo = tryGetInfo(); walletInfo == nil {
				select {
				case <-time.After(time.Second):
					continue
				case <-eco.outerCtx.Done():
					return
				}
			}
			break
		}

		// Delete the extraInput from the database, since it may contain
		// a password.
		if clearInput {
			eco.db.Store(extraInputKey, nil)
		}

		// I guess just run a loop to keep checking the connection for now.
		// Maybe should be checking the walletInfo.Blocks against dcrd's
		// reported tip height for progress, but not sure what to do in SPV
		// mode. I don't believe that dcrwallet offers any information via RPC
		// on wallet sync status.
		delay := time.Second * 5
		for {
			timer := time.NewTimer(delay)
			delay = time.Second * 5
			select {
			case <-timer.C:
				if walletInfo = tryGetInfo(); walletInfo == nil {
					timer.Stop()
					continue
				}

				delay = time.Second * 30

			case <-svcExe.ctx.Done():
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

	eco.stateMtx.Lock()
	defer eco.stateMtx.Unlock()

	if !atomic.CompareAndSwapUint32(&decreditonRunning, 0, 1) {
		return fmt.Errorf("Decrediton already running")
	}

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
		fmt.Sprintf("--custombinpath=%s", filepath.Join(programDirectory, eco.state.Eco.Version, decred)),
	}

	if eco.state.Eco.SyncMode == SyncModeSPV {
		args = append(args, "--spv")
	}

	exe := filepath.Join(programDirectory, eco.state.Eco.Version, decrediton, decreditonExeName)

	svcExe := newExe(eco.innerCtx, exe, args...)
	eco.dcrd.exe = svcExe

	go func() {
		defer atomic.StoreUint32(&decreditonRunning, 0)
		defer eco.sendServiceStatus(&ServiceStatus{
			Service: decrediton,
			On:      false,
		})

		fmt.Println("--Running Derediton")

		svcExe.Run()
	}()

	return nil
}

// func (eco *Eco) initializeDCRWallet(pw []byte) error {
// 	// There are two cases to consider.
// 	// 1. The wallet app data already exists, in which case we need to check
// 	//    the wallet password **after** dcrd is synced.
// 	// 2. The wallet app data doesn't exist yet. Create the wallet. Save the
// 	//    encrypted wallet seed until the user authorizes deletion.
// 	walletDBPath := filepath.Join(dcrwalletAppDir, "mainnet", "wallet.db")
// 	if fileExists(walletDBPath) {

// 	}
// }

// saveEcoState should be called with the stateMtx >= RLocked.
func (eco *Eco) saveEcoState() error {
	return eco.db.EncodeStore(ecoStateKey, &eco.state.Eco)
}

func fetchAsset(ctx context.Context, dir string, asset *releaseAsset) (string, error) {
	targetDir := filepath.Join(dir, asset.Name)
	archive, err := os.OpenFile(targetDir, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("Error creating file: %w", err)
	}
	defer archive.Close()

	// From github API docs...
	// > To download the asset's binary content, set the "Accept" header of the
	//   request to "application/octet-stream"
	client := &http.Client{}
	log.Infof("Fetching %q to %q", asset.URL, targetDir)
	req, err := http.NewRequestWithContext(ctx, "GET", asset.URL, nil)
	if err != nil {
		return "", fmt.Errorf("Error preparing request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Request error for %q %w", asset.URL, err)
	}

	defer resp.Body.Close()

	_, err = io.Copy(archive, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error saving archive to file: %v", err)
	}
	return archive.Name(), nil
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

func retrieveNetAddr() (*NetAddr, error) {
	path := filepath.Join(AppDir, ListenerFilename)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(b), " ")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid address file format")
	}
	return &NetAddr{
		Net:  parts[0],
		Addr: parts[1],
	}, nil
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
	for {
		fmt.Println("--getting a new feed")

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
				return fmt.Errorf("--function failure")
			}
		case <-ctx.Done():
			return nil
		}
	}

	return nil
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