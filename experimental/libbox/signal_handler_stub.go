//go:build !darwin || !badlinkname

package libbox

func PrepareCrashSignalHandlers() {}

func ReinstallCrashSignalHandlers() {}
