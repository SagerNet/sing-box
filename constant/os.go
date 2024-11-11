package constant

import (
	"github.com/sagernet/sing-box/constant/goos"
)

const IsAndroid = goos.IsAndroid == 1

const IsDarwin = goos.IsDarwin == 1 || goos.IsIos == 1

const IsDragonfly = goos.IsDragonfly == 1

const IsFreebsd = goos.IsFreebsd == 1

const IsHurd = goos.IsHurd == 1

const IsIllumos = goos.IsIllumos == 1

const IsIos = goos.IsIos == 1

const IsJs = goos.IsJs == 1

const IsLinux = goos.IsLinux == 1 || goos.IsAndroid == 1

const IsNacl = goos.IsNacl == 1

const IsNetbsd = goos.IsNetbsd == 1

const IsOpenbsd = goos.IsOpenbsd == 1

const IsPlan9 = goos.IsPlan9 == 1

const IsSolaris = goos.IsSolaris == 1

const IsWindows = goos.IsWindows == 1

const IsZos = goos.IsZos == 1
