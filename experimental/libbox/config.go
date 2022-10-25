package libbox

import (
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func parseConfig(configContent string) (option.Options, error) {
	var options option.Options
	err := options.UnmarshalJSON([]byte(configContent))
	if err != nil {
		return option.Options{}, E.Cause(err, "decode config")
	}
	return options, nil
}
