package url_test

import (
	_ "embed"
	"testing"

	"github.com/sagernet/sing-box/script/jstest"
	"github.com/sagernet/sing-box/script/modules/url"

	"github.com/dop251/goja"
)

var (
	//go:embed testdata/url_test.js
	urlTest string

	//go:embed testdata/url_search_params_test.js
	urlSearchParamsTest string
)

func TestURL(t *testing.T) {
	registry := jstest.NewRegistry()
	registry.RegisterNodeModule(url.ModuleName, url.Require)
	vm := goja.New()
	registry.Enable(vm)
	url.Enable(vm)
	vm.RunScript("url_test.js", urlTest)
}

func TestURLSearchParams(t *testing.T) {
	registry := jstest.NewRegistry()
	registry.RegisterNodeModule(url.ModuleName, url.Require)
	vm := goja.New()
	registry.Enable(vm)
	url.Enable(vm)
	vm.RunScript("url_search_params_test.js", urlSearchParamsTest)
}
