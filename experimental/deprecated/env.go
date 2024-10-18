package deprecated

import (
	"os"
	"strconv"

	"github.com/sagernet/sing/common/logger"
)

type envManager struct {
	logger logger.Logger
}

func NewEnvManager(logger logger.Logger) Manager {
	return &envManager{logger: logger}
}

func (f *envManager) ReportDeprecated(feature Note) {
	if !feature.Impending() {
		f.logger.Warn(feature.String())
		return
	}
	enable, enableErr := strconv.ParseBool(os.Getenv("ENABLE_DEPRECATED_" + feature.EnvName))
	if enableErr == nil && enable {
		f.logger.Warn(feature.String())
		return
	}
	f.logger.Error(feature.String())
	f.logger.Fatal("to continuing using this feature, set ENABLE_DEPRECATED_" + feature.EnvName + "=true")
}
