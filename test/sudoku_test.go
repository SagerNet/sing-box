package main

import (
	"net/netip"
	"runtime"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	sudokut "github.com/sagernet/sing-box/transport/sudoku"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

func testSudokuSelf(t *testing.T, inboundOptions option.SudokuInboundOptions, outboundOptions option.SudokuOutboundOptions) {
	inboundOptions.ListenOptions = option.ListenOptions{
		Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
		ListenPort: serverPort,
	}
	outboundOptions.Server = "127.0.0.1"
	outboundOptions.ServerPort = serverPort

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type:    C.TypeSudoku,
				Tag:     "sudoku-in",
				Options: &inboundOptions,
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type:    C.TypeSudoku,
				Tag:     "sudoku-out",
				Options: &outboundOptions,
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "sudoku-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestSudoku_Basic(t *testing.T) {
	key := "test_key"
	t.Run("plain", func(t *testing.T) {
		in := option.SudokuInboundOptions{
			Key: key,
		}
		out := option.SudokuOutboundOptions{
			Key: key,
		}
		testSudokuSelf(t, in, out)
	})

	t.Run("ed25519", func(t *testing.T) {
		privateKey, publicKey, err := sudokut.GenKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		in := option.SudokuInboundOptions{
			Key: publicKey,
		}
		out := option.SudokuOutboundOptions{
			Key: privateKey,
		}
		testSudokuSelf(t, in, out)
	})
}

func TestSudoku_Entropy(t *testing.T) {
	key := "test_key_entropy"
	in := option.SudokuInboundOptions{
		Key:   key,
		ASCII: "prefer_entropy",
	}
	out := option.SudokuOutboundOptions{
		Key:   key,
		ASCII: "prefer_entropy",
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_Padding(t *testing.T) {
	key := "test_key_padding"
	paddingMin := 1
	paddingMax := 9
	in := option.SudokuInboundOptions{
		Key:        key,
		PaddingMin: &paddingMin,
		PaddingMax: &paddingMax,
	}
	out := option.SudokuOutboundOptions{
		Key:        key,
		PaddingMin: &paddingMin,
		PaddingMax: &paddingMax,
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_PackedDownlink(t *testing.T) {
	key := "test_key_packed"
	enablePure := false
	in := option.SudokuInboundOptions{
		Key:                key,
		EnablePureDownlink: &enablePure,
	}
	out := option.SudokuOutboundOptions{
		Key:                key,
		EnablePureDownlink: &enablePure,
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_CustomTable(t *testing.T) {
	key := "test_key_custom"
	custom := "xpxvvpvv"
	in := option.SudokuInboundOptions{
		Key:         key,
		ASCII:       "prefer_entropy",
		CustomTable: custom,
	}
	out := option.SudokuOutboundOptions{
		Key:         key,
		ASCII:       "prefer_entropy",
		CustomTable: custom,
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_TableRotation(t *testing.T) {
	key := "test_key_rotate"
	patterns := []string{"xpxvvpvv", "xxppvvvv"}
	in := option.SudokuInboundOptions{
		Key:          key,
		ASCII:        "prefer_entropy",
		CustomTables: patterns,
	}
	out := option.SudokuOutboundOptions{
		Key:          key,
		ASCII:        "prefer_entropy",
		CustomTables: patterns,
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_AEADNone(t *testing.T) {
	key := "test_key_none"
	in := option.SudokuInboundOptions{
		Key:        key,
		AEADMethod: "none",
	}
	out := option.SudokuOutboundOptions{
		Key:        key,
		AEADMethod: "none",
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_DisableHTTPMask(t *testing.T) {
	key := "test_key_no_mask"
	in := option.SudokuInboundOptions{
		Key:             key,
		DisableHTTPMask: true,
	}
	out := option.SudokuOutboundOptions{
		Key:             key,
		DisableHTTPMask: true,
	}
	testSudokuSelf(t, in, out)
}

func TestSudoku_HTTPMaskStrategy(t *testing.T) {
	key := "test_key_mask_strategy"
	for _, strategy := range []string{"random", "post", "websocket"} {
		strategy := strategy
		t.Run(strategy, func(t *testing.T) {
			in := option.SudokuInboundOptions{
				Key: key,
			}
			out := option.SudokuOutboundOptions{
				Key:              key,
				HTTPMaskStrategy: strategy,
			}
			testSudokuSelf(t, in, out)
		})
	}
}

func TestSudoku_HTTPMaskMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("intermittent failures on windows")
	}

	key := "test_key_http_mask_mode"
	for _, mode := range []string{"legacy", "stream", "poll", "auto"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			in := option.SudokuInboundOptions{
				Key:          key,
				HTTPMaskMode: mode,
			}
			out := option.SudokuOutboundOptions{
				Key:          key,
				HTTPMaskMode: mode,
			}
			testSudokuSelf(t, in, out)
		})
	}
}
