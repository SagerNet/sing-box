package libbox

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/networkquality"
	"github.com/sagernet/sing-box/common/stun"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/experimental/locale"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	sBasePath                string
	sWorkingPath             string
	sTempPath                string
	sUserID                  int
	sGroupID                 int
	sFixAndroidStack         bool
	sCommandServerListenPort uint16
	sCommandServerSecret     string
	sLogMaxLines             int
	sDebug                   bool
	sCrashReportSource       string
	sOOMKillerEnabled        bool
	sOOMKillerDisabled       bool
	sOOMMemoryLimit          int64
)

func init() {
	debug.SetPanicOnFault(true)
	debug.SetTraceback("all")
}

type SetupOptions struct {
	BasePath                string
	WorkingPath             string
	TempPath                string
	FixAndroidStack         bool
	CommandServerListenPort int32
	CommandServerSecret     string
	LogMaxLines             int
	Debug                   bool
	CrashReportSource       string
	OomKillerEnabled        bool
	OomKillerDisabled       bool
	OomMemoryLimit          int64
}

func applySetupOptions(options *SetupOptions) {
	sBasePath = options.BasePath
	sWorkingPath = options.WorkingPath
	sTempPath = options.TempPath

	sUserID = os.Getuid()
	sGroupID = os.Getgid()

	// TODO: remove after fixed
	// https://github.com/golang/go/issues/68760
	sFixAndroidStack = options.FixAndroidStack

	sCommandServerListenPort = uint16(options.CommandServerListenPort)
	sCommandServerSecret = options.CommandServerSecret
	sLogMaxLines = options.LogMaxLines
	sDebug = options.Debug
	sCrashReportSource = options.CrashReportSource
	ReloadSetupOptions(options)
}

func ReloadSetupOptions(options *SetupOptions) {
	sOOMKillerEnabled = options.OomKillerEnabled
	sOOMKillerDisabled = options.OomKillerDisabled
	sOOMMemoryLimit = options.OomMemoryLimit
	if sOOMKillerEnabled {
		if sOOMMemoryLimit == 0 && C.IsIos {
			sOOMMemoryLimit = oomkiller.DefaultAppleNetworkExtensionMemoryLimit
		}
		if sOOMMemoryLimit > 0 {
			debug.SetMemoryLimit(sOOMMemoryLimit * 3 / 4)
		} else {
			debug.SetMemoryLimit(math.MaxInt64)
		}
	} else {
		debug.SetMemoryLimit(math.MaxInt64)
	}
}

func Setup(options *SetupOptions) error {
	applySetupOptions(options)
	os.MkdirAll(sWorkingPath, 0o777)
	os.MkdirAll(sTempPath, 0o777)
	return redirectStderr(filepath.Join(sWorkingPath, "CrashReport-"+sCrashReportSource+".log"))
}

func SetLocale(localeId string) error {
	if strings.Contains(localeId, "@") {
		localeId = strings.Split(localeId, "@")[0]
	}
	if !locale.Set(localeId) {
		return E.New("unsupported locale: ", localeId)
	}
	return nil
}

func Version() string {
	return C.Version
}

func GoVersion() string {
	return runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH
}

func FormatBytes(length int64) string {
	return byteformats.FormatKBytes(uint64(length))
}

func FormatMemoryBytes(length int64) string {
	return byteformats.FormatMemoryKBytes(uint64(length))
}

func FormatDuration(duration int64) string {
	return log.FormatDuration(time.Duration(duration) * time.Millisecond)
}

func FormatBitrate(bps int64) string {
	return networkquality.FormatBitrate(bps)
}

const NetworkQualityDefaultConfigURL = networkquality.DefaultConfigURL

const NetworkQualityDefaultMaxRuntimeSeconds = int32(networkquality.DefaultMaxRuntime / time.Second)

const (
	NetworkQualityAccuracyLow    = int32(networkquality.AccuracyLow)
	NetworkQualityAccuracyMedium = int32(networkquality.AccuracyMedium)
	NetworkQualityAccuracyHigh   = int32(networkquality.AccuracyHigh)
)

const (
	NetworkQualityPhaseIdle     = int32(networkquality.PhaseIdle)
	NetworkQualityPhaseDownload = int32(networkquality.PhaseDownload)
	NetworkQualityPhaseUpload   = int32(networkquality.PhaseUpload)
	NetworkQualityPhaseDone     = int32(networkquality.PhaseDone)
)

const STUNDefaultServer = stun.DefaultServer

const (
	STUNPhaseBinding      = int32(stun.PhaseBinding)
	STUNPhaseNATMapping   = int32(stun.PhaseNATMapping)
	STUNPhaseNATFiltering = int32(stun.PhaseNATFiltering)
	STUNPhaseDone         = int32(stun.PhaseDone)
)

const (
	NATMappingEndpointIndependent     = int32(stun.NATMappingEndpointIndependent)
	NATMappingAddressDependent        = int32(stun.NATMappingAddressDependent)
	NATMappingAddressAndPortDependent = int32(stun.NATMappingAddressAndPortDependent)
)

const (
	NATFilteringEndpointIndependent     = int32(stun.NATFilteringEndpointIndependent)
	NATFilteringAddressDependent        = int32(stun.NATFilteringAddressDependent)
	NATFilteringAddressAndPortDependent = int32(stun.NATFilteringAddressAndPortDependent)
)

func FormatNATMapping(value int32) string {
	return stun.NATMapping(value).String()
}

func FormatNATFiltering(value int32) string {
	return stun.NATFiltering(value).String()
}

func FormatFQDN(fqdn string) string {
	return dns.FqdnToDomain(fqdn)
}

func ProxyDisplayType(proxyType string) string {
	return C.ProxyDisplayName(proxyType)
}
