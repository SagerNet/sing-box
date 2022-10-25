package libbox

type PlatformInterface interface {
	AutoDetectInterfaceControl(fd int32) error
	OpenTun(options TunOptions) (TunInterface, error)
	WriteLog(message string)
	UseProcFS() bool
	FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (int32, error)
	PackageNameByUid(uid int32) (string, error)
	UIDByPackageName(packageName string) (int32, error)
}

type TunInterface interface {
	FileDescriptor() int32
	Close() error
}
