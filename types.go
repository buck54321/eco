package eco

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
