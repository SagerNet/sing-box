package nekoutils

import "github.com/sagernet/sing-box/option"

var GetGeoIPHeadlessRules func(name string) ([]option.HeadlessRule, error)

var GetGeoSiteHeadlessRules func(name string) ([]option.HeadlessRule, error)
