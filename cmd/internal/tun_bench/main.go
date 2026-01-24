package main

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/shell"
)

var iperf3Path string

func main() {
	err := main0()
	if err != nil {
		log.Fatal(err)
	}
}

func main0() error {
	err := shell.Exec("sudo", "ls").Run()
	if err != nil {
		return err
	}
	results, err := runTests()
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func runTests() ([]TestResult, error) {
	boxPaths := []string{
		os.ExpandEnv("$HOME/Downloads/sing-box-1.11.15-darwin-arm64/sing-box"),
		//"/Users/sekai/Downloads/sing-box-1.11.15-linux-arm64/sing-box",
		"./sing-box",
	}
	stacks := []string{
		"gvisor",
		"system",
	}
	mtus := []int{
		1500,
		4064,
		// 16384,
		// 32768,
		// 49152,
		65535,
	}
	flagList := [][]string{
		{},
	}
	var results []TestResult
	for _, boxPath := range boxPaths {
		for _, stack := range stacks {
			for _, mtu := range mtus {
				if strings.HasPrefix(boxPath, ".") {
					for _, flags := range flagList {
						result, err := testOnce(boxPath, stack, mtu, false, flags)
						if err != nil {
							return nil, err
						}
						results = append(results, *result)
					}
				} else {
					result, err := testOnce(boxPath, stack, mtu, false, nil)
					if err != nil {
						return nil, err
					}
					results = append(results, *result)
				}
			}
		}
	}
	return results, nil
}

type TestResult struct {
	BoxPath       string   `json:"box_path"`
	Stack         string   `json:"stack"`
	MTU           int      `json:"mtu"`
	Flags         []string `json:"flags"`
	MultiThread   bool     `json:"multi_thread"`
	UploadSpeed   string   `json:"upload_speed"`
	DownloadSpeed string   `json:"download_speed"`
}

func testOnce(boxPath string, stackName string, mtu int, multiThread bool, flags []string) (result *TestResult, err error) {
	testAddress := netip.MustParseAddr("1.1.1.1")
	testConfig := option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeTun,
				Options: &option.TunInboundOptions{
					Address:      []netip.Prefix{netip.MustParsePrefix("172.18.0.1/30")},
					AutoRoute:    true,
					MTU:          uint32(mtu),
					Stack:        stackName,
					RouteAddress: []netip.Prefix{netip.PrefixFrom(testAddress, testAddress.BitLen())},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							IPCIDR: []string{testAddress.String()},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRouteOptions,
							RouteOptionsOptions: option.RouteOptionsActionOptions{
								OverrideAddress: "127.0.0.1",
							},
						},
					},
				},
			},
			AutoDetectInterface: true,
		},
	}
	ctx := include.Context(context.Background())
	tempConfig, err := os.CreateTemp("", "tun-bench-*.json")
	if err != nil {
		return
	}
	defer os.Remove(tempConfig.Name())
	encoder := json.NewEncoderContext(ctx, tempConfig)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(testConfig)
	if err != nil {
		return nil, E.Cause(err, "encode test config")
	}
	tempConfig.Close()
	var sudoArgs []string
	if len(flags) > 0 {
		sudoArgs = append(sudoArgs, "env")
		sudoArgs = append(sudoArgs, flags...)
	}
	sudoArgs = append(sudoArgs, boxPath, "run", "-c", tempConfig.Name())
	boxProcess := shell.Exec("sudo", sudoArgs...)
	boxProcess.Stdout = &stderrWriter{}
	boxProcess.Stderr = io.Discard
	err = boxProcess.Start()
	if err != nil {
		return
	}

	if C.IsDarwin {
		iperf3Path, err = exec.LookPath("iperf3-darwin")
	} else {
		iperf3Path, err = exec.LookPath("iperf3")
	}
	if err != nil {
		return
	}
	serverProcess := shell.Exec(iperf3Path, "-s")
	serverProcess.Stdout = io.Discard
	serverProcess.Stderr = io.Discard
	err = serverProcess.Start()
	if err != nil {
		return nil, E.Cause(err, "start iperf3 server")
	}

	time.Sleep(time.Second)

	args := []string{"-c", testAddress.String()}
	if multiThread {
		args = append(args, "-P", "10")
	}

	uploadProcess := shell.Exec(iperf3Path, args...)
	output, err := uploadProcess.Read()
	if err != nil {
		boxProcess.Process.Signal(syscall.SIGKILL)
		serverProcess.Process.Signal(syscall.SIGKILL)
		println(output)
		return
	}

	uploadResult := common.SubstringBeforeLast(output, "iperf Done.")
	uploadResult = common.SubstringBeforeLast(uploadResult, "sender")
	uploadResult = common.SubstringBeforeLast(uploadResult, "bits/sec")
	uploadResult = common.SubstringAfterLast(uploadResult, "Bytes")
	uploadResult = strings.ReplaceAll(uploadResult, " ", "")

	result = &TestResult{
		BoxPath:     boxPath,
		Stack:       stackName,
		MTU:         mtu,
		Flags:       flags,
		MultiThread: multiThread,
		UploadSpeed: uploadResult,
	}

	downloadProcess := shell.Exec(iperf3Path, append(args, "-R")...)
	output, err = downloadProcess.Read()
	if err != nil {
		boxProcess.Process.Signal(syscall.SIGKILL)
		serverProcess.Process.Signal(syscall.SIGKILL)
		println(output)
		return
	}

	downloadResult := common.SubstringBeforeLast(output, "iperf Done.")
	downloadResult = common.SubstringBeforeLast(downloadResult, "receiver")
	downloadResult = common.SubstringBeforeLast(downloadResult, "bits/sec")
	downloadResult = common.SubstringAfterLast(downloadResult, "Bytes")
	downloadResult = strings.ReplaceAll(downloadResult, " ", "")

	result.DownloadSpeed = downloadResult

	printArgs := []any{boxPath, stackName, mtu, "upload", uploadResult, "download", downloadResult}
	if len(flags) > 0 {
		printArgs = append(printArgs, "flags", strings.Join(flags, " "))
	}
	if multiThread {
		printArgs = append(printArgs, "(-P 10)")
	}
	fmt.Println(printArgs...)
	err = boxProcess.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return
	}

	err = serverProcess.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return
	}

	boxDone := make(chan struct{})
	go func() {
		boxProcess.Cmd.Wait()
		close(boxDone)
	}()

	serverDone := make(chan struct{})
	go func() {
		serverProcess.Process.Wait()
		close(serverDone)
	}()

	select {
	case <-boxDone:
	case <-time.After(2 * time.Second):
		boxProcess.Process.Kill()
	case <-time.After(4 * time.Second):
		println("box process did not close!")
		os.Exit(1)
	}

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		serverProcess.Process.Kill()
	case <-time.After(4 * time.Second):
		println("server process did not close!")
		os.Exit(1)
	}

	return
}

type stderrWriter struct{}

func (w *stderrWriter) Write(p []byte) (n int, err error) {
	return os.Stderr.Write(p)
}
