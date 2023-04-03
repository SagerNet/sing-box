//go:build linux || darwin

package libbox

import (
	"context"

	"github.com/sagernet/sing-box"
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

func CheckConfig(configContent string) error {
	options, err := parseConfig(configContent)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err == nil {
		instance.Close()
	}
	return err
}
