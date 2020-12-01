package eco

import (
	"github.com/buck54321/eco/encode"
	"github.com/buck54321/eco/encrypt"
)

type EcoError struct {
	Msg string
}

func (e *EcoError) Errorf() string {
	return e.Msg
}

type SyncMode uint8

const (
	SyncModeInvalid SyncMode = iota
	SyncModeUninitialized
	SyncModeSPV
	SyncModeFull
)

type MetaState struct {
	Eco      EcoState
	Services map[string]*ServiceStatus
}

type EcoState struct {
	SyncMode     SyncMode
	WalletExists bool
	Version      string
}

type DCRDState struct {
	UserSettings DCRDUserSettings
	RPCUser      string
	RPCPass      string
}

type DCRDUserSettings struct {
	DebugLevel string
}

type DCRWalletState struct {
	Status ServiceStatus
}

type Progress struct {
	Service  string
	Status   string
	Err      string
	Progress float32
}

type ServiceStatus struct {
	Service string
	On      bool
}

// There are a couple of times when a password needs to be saved to disk
// temporarily. In that case, encrypt the password and save it encrypted
// password with the encryption key. Is it secure? No. But at least we're not
// storing plain-text passwords. I'll work with dcrwallet and dcrdex to see if
// we can get around these needs.
type pwCache struct {
	EncPW   []byte
	Key     []byte
	Crypter []byte
}

func newPWCache(pw []byte) (*pwCache, error) {
	encKey := encode.RandomBytes(32)
	tmpCrypter := encrypt.NewCrypter(encKey)
	encPW, err := tmpCrypter.Encrypt(pw)
	if err != nil {
		return nil, err
	}
	return &pwCache{
		EncPW:   encPW,
		Key:     encKey,
		Crypter: tmpCrypter.Serialize(),
	}, nil
}

func (pwc *pwCache) PW() ([]byte, error) {
	tmpCrypter, err := encrypt.Deserialize(pwc.Key, pwc.Crypter)
	if err != nil {
		return nil, err
	}
	pw, err := tmpCrypter.Decrypt(pwc.EncPW)
	if err != nil {
		return nil, err
	}
	return pw, nil
}
