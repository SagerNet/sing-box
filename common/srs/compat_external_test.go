package srs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

func TestCompatWithSingBox117(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	rootDirectory, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	baseDirectory, err := os.MkdirTemp("", "sing-box-compat-")
	if err != nil {
		t.Fatalf("make temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(baseDirectory)
	})
	cacheDirectory := filepath.Join(baseDirectory, "cache")
	workDirectory := filepath.Join(baseDirectory, "work")
	geoipPath := ensureDatabase(t, cacheDirectory, filepath.Join(rootDirectory, "geoip.db"), "sagernet/sing-geoip", "geoip.db")
	geositePath := ensureDatabase(t, cacheDirectory, filepath.Join(rootDirectory, "geosite.db"), "sagernet/sing-geosite", "geosite.db")
	if err := os.MkdirAll(workDirectory, 0o755); err != nil {
		t.Fatalf("make work dir: %v", err)
	}
	if err := copyFile(geoipPath, filepath.Join(workDirectory, "geoip.db")); err != nil {
		t.Fatalf("copy geoip.db: %v", err)
	}
	if err := copyFile(geositePath, filepath.Join(workDirectory, "geosite.db")); err != nil {
		t.Fatalf("copy geosite.db: %v", err)
	}

	oldBinary, err := ensureSingBox117(cacheDirectory)
	if err != nil {
		t.Fatalf("prepare sing-box 1.12.17: %v", err)
	}

	runCommand(t, workDirectory, oldBinary, "geoip", "export", "cn")
	runCommand(t, workDirectory, oldBinary, "rule-set", "compile", "geoip-cn.json")
	assertRuleSetHasIPCIDR(t, filepath.Join(workDirectory, "geoip-cn.srs"))

	runCommand(t, workDirectory, oldBinary, "geosite", "export", "geolocation-cn")
	runCommand(t, workDirectory, oldBinary, "rule-set", "compile", "geosite-geolocation-cn.json")
	assertRuleSetHasDomain(t, filepath.Join(workDirectory, "geosite-geolocation-cn.srs"))

	adguardPath := filepath.Join(workDirectory, "adguard.txt")
	if err := os.WriteFile(adguardPath, []byte("||ads.example.com^\n||tracker.example.org^\n"), 0o644); err != nil {
		t.Fatalf("write adguard file: %v", err)
	}
	runCommand(t, workDirectory, oldBinary, "rule-set", "convert", "-t", "adguard", adguardPath)
	assertRuleSetHasAdGuard(t, filepath.Join(workDirectory, "adguard.srs"))

	verifyOldBinaryReadsNewRuleSet(t, oldBinary, workDirectory)
	verifyOldBinaryMatchesAdGuardRuleSet(t, oldBinary, workDirectory)
}

func verifyOldBinaryReadsNewRuleSet(t *testing.T, oldBinary, workDirectory string) {
	t.Helper()
	rule := option.HeadlessRule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			QueryType: badoption.Listable[option.DNSQueryType]{
				option.DNSQueryType(1),
				option.DNSQueryType(28),
			},
			Network:       badoption.Listable[string]{"tcp", "udp"},
			Domain:        badoption.Listable[string]{"example.com", "example.org"},
			DomainSuffix:  badoption.Listable[string]{".example.net"},
			DomainKeyword: badoption.Listable[string]{"keyword"},
			DomainRegex:   badoption.Listable[string]{`^.*\.regex\.test$`},
			SourceIPCIDR:  badoption.Listable[string]{"10.0.0.0/8"},
			IPCIDR: badoption.Listable[string]{
				"1.2.3.0/24",
				"2001:db8::/32",
			},
			SourcePort:      badoption.Listable[uint16]{53},
			SourcePortRange: badoption.Listable[string]{"1000-2000"},
			Port:            badoption.Listable[uint16]{80, 443},
			PortRange:       badoption.Listable[string]{"3000-4000"},
			ProcessName:     badoption.Listable[string]{"proc"},
			ProcessPath:     badoption.Listable[string]{"/usr/bin/proc"},
			ProcessPathRegex: badoption.Listable[string]{
				`^/usr/bin/.*$`,
			},
			PackageName:          badoption.Listable[string]{"com.example.app"},
			WIFISSID:             badoption.Listable[string]{"TestWiFi"},
			WIFIBSSID:            badoption.Listable[string]{"00:11:22:33:44:55"},
			NetworkType:          badoption.Listable[option.InterfaceType]{option.InterfaceType(C.InterfaceTypeWIFI), option.InterfaceType(C.InterfaceTypeCellular)},
			NetworkIsExpensive:   true,
			NetworkIsConstrained: true,
			Invert:               true,
		},
	}
	logicalRule := option.HeadlessRule{
		Type: C.RuleTypeLogical,
		LogicalOptions: option.LogicalHeadlessRule{
			Mode: C.LogicalTypeOr,
			Rules: []option.HeadlessRule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultHeadlessRule{
						Domain: badoption.Listable[string]{"logic.example.com"},
					},
				},
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultHeadlessRule{
						IPCIDR: badoption.Listable[string]{"203.0.113.0/24"},
					},
				},
			},
			Invert: true,
		},
	}
	ruleSet := option.PlainRuleSet{
		Rules: []option.HeadlessRule{rule, logicalRule},
	}
	outputPath := filepath.Join(workDirectory, "current.srs")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("create srs: %v", err)
	}
	if err := Write(outputFile, ruleSet, C.RuleSetVersion3); err != nil {
		outputFile.Close()
		t.Fatalf("write srs: %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("close srs: %v", err)
	}
	outputJSON := filepath.Join(workDirectory, "current.json")
	runCommand(t, workDirectory, oldBinary, "rule-set", "decompile", outputPath, "-o", outputJSON)
	compat := readRuleSetJSON(t, outputJSON)
	if len(compat.Options.Rules) != 2 {
		t.Fatalf("unexpected rules length: %d", len(compat.Options.Rules))
	}
	assertDefaultRuleMatches(t, rule.DefaultOptions, compat.Options.Rules[0].DefaultOptions)
	assertLogicalRuleMatches(t, logicalRule.LogicalOptions, compat.Options.Rules[1].LogicalOptions)
}

func verifyOldBinaryMatchesAdGuardRuleSet(t *testing.T, oldBinary, workDirectory string) {
	t.Helper()
	rule := option.HeadlessRule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			AdGuardDomain: badoption.Listable[string]{"||ads.example.com^"},
		},
	}
	ruleSet := option.PlainRuleSet{
		Rules: []option.HeadlessRule{rule},
	}
	outputPath := filepath.Join(workDirectory, "adguard-current.srs")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("create adguard srs: %v", err)
	}
	if err := Write(outputFile, ruleSet, C.RuleSetVersion2); err != nil {
		outputFile.Close()
		t.Fatalf("write adguard srs: %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("close adguard srs: %v", err)
	}
	output, err := runCommandOutput(workDirectory, oldBinary, "rule-set", "match", "-f", "binary", outputPath, "ads.example.com")
	if err != nil {
		t.Fatalf("match adguard rule-set: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "match rules.[0]") {
		t.Fatalf("expected adguard match output, got: %s", output)
	}
}

func assertRuleSetHasIPCIDR(t *testing.T, path string) {
	t.Helper()
	ruleSet := readRuleSet(t, path)
	if !hasRule(ruleSet.Options.Rules, func(rule option.DefaultHeadlessRule) bool {
		return len(rule.IPCIDR) > 0 || len(rule.SourceIPCIDR) > 0
	}) {
		t.Fatalf("rule-set missing ipcidr: %s", path)
	}
}

func assertRuleSetHasDomain(t *testing.T, path string) {
	t.Helper()
	ruleSet := readRuleSet(t, path)
	if !hasRule(ruleSet.Options.Rules, func(rule option.DefaultHeadlessRule) bool {
		return len(rule.Domain) > 0 || len(rule.DomainSuffix) > 0 || len(rule.DomainKeyword) > 0 || len(rule.DomainRegex) > 0
	}) {
		t.Fatalf("rule-set missing domain data: %s", path)
	}
}

func assertRuleSetHasAdGuard(t *testing.T, path string) {
	t.Helper()
	ruleSet := readRuleSet(t, path)
	if !hasRule(ruleSet.Options.Rules, func(rule option.DefaultHeadlessRule) bool {
		return len(rule.AdGuardDomain) > 0
	}) {
		t.Fatalf("rule-set missing adguard data: %s", path)
	}
	if !hasRule(ruleSet.Options.Rules, func(rule option.DefaultHeadlessRule) bool {
		return containsSubstring(rule.AdGuardDomain, "ads.example.com")
	}) {
		t.Fatalf("rule-set missing adguard content: %s", path)
	}
}

func readRuleSet(t *testing.T, path string) option.PlainRuleSetCompat {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open rule-set: %v", err)
	}
	defer file.Close()
	ruleSet, err := Read(file, true)
	if err != nil {
		t.Fatalf("read rule-set: %v", err)
	}
	if len(ruleSet.Options.Rules) == 0 {
		t.Fatalf("rule-set empty: %s", path)
	}
	return ruleSet
}

func readRuleSetJSON(t *testing.T, path string) option.PlainRuleSetCompat {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rule-set json: %v", err)
	}
	ruleSet, err := singjson.UnmarshalExtended[option.PlainRuleSetCompat](content)
	if err != nil {
		t.Fatalf("unmarshal rule-set json: %v", err)
	}
	return ruleSet
}

func assertDefaultRuleMatches(t *testing.T, expected, actual option.DefaultHeadlessRule) {
	t.Helper()
	if !sameUint16Set(queryTypesToUint16(expected.QueryType), queryTypesToUint16(actual.QueryType)) {
		t.Fatalf("query_type mismatch: want=%v got=%v", expected.QueryType, actual.QueryType)
	}
	if !sameStringSet(expected.Network, actual.Network) {
		t.Fatalf("network mismatch: want=%v got=%v", expected.Network, actual.Network)
	}
	if !sameStringSet(expected.Domain, actual.Domain) {
		t.Fatalf("domain mismatch: want=%v got=%v", expected.Domain, actual.Domain)
	}
	if !sameStringSet(expected.DomainSuffix, actual.DomainSuffix) {
		t.Fatalf("domain_suffix mismatch: want=%v got=%v", expected.DomainSuffix, actual.DomainSuffix)
	}
	if !sameStringSet(expected.DomainKeyword, actual.DomainKeyword) {
		t.Fatalf("domain_keyword mismatch: want=%v got=%v", expected.DomainKeyword, actual.DomainKeyword)
	}
	if !sameStringSet(expected.DomainRegex, actual.DomainRegex) {
		t.Fatalf("domain_regex mismatch: want=%v got=%v", expected.DomainRegex, actual.DomainRegex)
	}
	if !sameStringSet(expected.SourceIPCIDR, actual.SourceIPCIDR) {
		t.Fatalf("source_ip_cidr mismatch: want=%v got=%v", expected.SourceIPCIDR, actual.SourceIPCIDR)
	}
	if !sameStringSet(expected.IPCIDR, actual.IPCIDR) {
		t.Fatalf("ip_cidr mismatch: want=%v got=%v", expected.IPCIDR, actual.IPCIDR)
	}
	if !sameUint16Set(expected.SourcePort, actual.SourcePort) {
		t.Fatalf("source_port mismatch: want=%v got=%v", expected.SourcePort, actual.SourcePort)
	}
	if !sameStringSet(expected.SourcePortRange, actual.SourcePortRange) {
		t.Fatalf("source_port_range mismatch: want=%v got=%v", expected.SourcePortRange, actual.SourcePortRange)
	}
	if !sameUint16Set(expected.Port, actual.Port) {
		t.Fatalf("port mismatch: want=%v got=%v", expected.Port, actual.Port)
	}
	if !sameStringSet(expected.PortRange, actual.PortRange) {
		t.Fatalf("port_range mismatch: want=%v got=%v", expected.PortRange, actual.PortRange)
	}
	if !sameStringSet(expected.ProcessName, actual.ProcessName) {
		t.Fatalf("process_name mismatch: want=%v got=%v", expected.ProcessName, actual.ProcessName)
	}
	if !sameStringSet(expected.ProcessPath, actual.ProcessPath) {
		t.Fatalf("process_path mismatch: want=%v got=%v", expected.ProcessPath, actual.ProcessPath)
	}
	if !sameStringSet(expected.ProcessPathRegex, actual.ProcessPathRegex) {
		t.Fatalf("process_path_regex mismatch: want=%v got=%v", expected.ProcessPathRegex, actual.ProcessPathRegex)
	}
	if !sameStringSet(expected.PackageName, actual.PackageName) {
		t.Fatalf("package_name mismatch: want=%v got=%v", expected.PackageName, actual.PackageName)
	}
	if !sameStringSet(expected.WIFISSID, actual.WIFISSID) {
		t.Fatalf("wifi_ssid mismatch: want=%v got=%v", expected.WIFISSID, actual.WIFISSID)
	}
	if !sameStringSet(expected.WIFIBSSID, actual.WIFIBSSID) {
		t.Fatalf("wifi_bssid mismatch: want=%v got=%v", expected.WIFIBSSID, actual.WIFIBSSID)
	}
	if !sameInterfaceTypeSet(expected.NetworkType, actual.NetworkType) {
		t.Fatalf("network_type mismatch: want=%v got=%v", expected.NetworkType, actual.NetworkType)
	}
	if expected.NetworkIsExpensive != actual.NetworkIsExpensive {
		t.Fatalf("network_is_expensive mismatch: want=%v got=%v", expected.NetworkIsExpensive, actual.NetworkIsExpensive)
	}
	if expected.NetworkIsConstrained != actual.NetworkIsConstrained {
		t.Fatalf("network_is_constrained mismatch: want=%v got=%v", expected.NetworkIsConstrained, actual.NetworkIsConstrained)
	}
	if expected.Invert != actual.Invert {
		t.Fatalf("invert mismatch: want=%v got=%v", expected.Invert, actual.Invert)
	}
}

func assertLogicalRuleMatches(t *testing.T, expected, actual option.LogicalHeadlessRule) {
	t.Helper()
	if expected.Mode != actual.Mode {
		t.Fatalf("logical mode mismatch: want=%s got=%s", expected.Mode, actual.Mode)
	}
	if expected.Invert != actual.Invert {
		t.Fatalf("logical invert mismatch: want=%v got=%v", expected.Invert, actual.Invert)
	}
	if len(actual.Rules) != len(expected.Rules) {
		t.Fatalf("logical rules length mismatch: want=%d got=%d", len(expected.Rules), len(actual.Rules))
	}
}

func queryTypesToUint16(values []option.DNSQueryType) []uint16 {
	result := make([]uint16, 0, len(values))
	for _, value := range values {
		result = append(result, uint16(value))
	}
	return result
}

func sameInterfaceTypeSet(a, b []option.InterfaceType) bool {
	if len(a) != len(b) {
		return false
	}
	left := make([]uint8, 0, len(a))
	right := make([]uint8, 0, len(b))
	for _, value := range a {
		left = append(left, uint8(value))
	}
	for _, value := range b {
		right = append(right, uint8(value))
	}
	return sameUint8Set(left, right)
}

func sameUint8Set(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]uint8(nil), a...)
	bCopy := append([]uint8(nil), b...)
	sort.Slice(aCopy, func(i, j int) bool { return aCopy[i] < aCopy[j] })
	sort.Slice(bCopy, func(i, j int) bool { return bCopy[i] < bCopy[j] })
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}

func sameUint16Set(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]uint16(nil), a...)
	bCopy := append([]uint16(nil), b...)
	sort.Slice(aCopy, func(i, j int) bool { return aCopy[i] < aCopy[j] })
	sort.Slice(bCopy, func(i, j int) bool { return bCopy[i] < bCopy[j] })
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}

func containsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func hasRule(rules []option.HeadlessRule, cond func(rule option.DefaultHeadlessRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			if hasRule(rule.LogicalOptions.Rules, cond) {
				return true
			}
		}
	}
	return false
}

func ensureSingBox117(cacheDirectory string) (string, error) {
	if err := os.MkdirAll(cacheDirectory, 0o755); err != nil {
		return "", err
	}
	binaryPath := filepath.Join(cacheDirectory, binaryName())
	if isExecutable(binaryPath) {
		return binaryPath, nil
	}
	assetName, err := assetName()
	if err != nil {
		return "", err
	}
	archivePath := filepath.Join(cacheDirectory, assetName)
	if !fileExists(archivePath) {
		if err := downloadReleaseAsset(assetName, cacheDirectory); err != nil {
			return "", err
		}
	}
	extractDirectory := filepath.Join(cacheDirectory, "extract")
	if err := os.MkdirAll(extractDirectory, 0o755); err != nil {
		return "", err
	}
	if strings.HasSuffix(assetName, ".tar.gz") {
		if err := extractTarGz(archivePath, extractDirectory); err != nil {
			return "", err
		}
	} else if strings.HasSuffix(assetName, ".zip") {
		if err := extractZip(archivePath, extractDirectory); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("unsupported archive: %s", assetName)
	}
	foundBinary, err := findBinary(extractDirectory)
	if err != nil {
		return "", err
	}
	if err := copyFile(foundBinary, binaryPath); err != nil {
		return "", err
	}
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func assetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "sing-box-1.12.17-darwin-amd64.tar.gz", nil
		case "arm64":
			return "sing-box-1.12.17-darwin-arm64.tar.gz", nil
		default:
			return "", fmt.Errorf("unsupported darwin arch: %s", runtime.GOARCH)
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "sing-box-1.12.17-linux-amd64.tar.gz", nil
		case "arm64":
			return "sing-box-1.12.17-linux-arm64.tar.gz", nil
		case "386":
			return "sing-box-1.12.17-linux-386.tar.gz", nil
		default:
			return "", fmt.Errorf("unsupported linux arch: %s", runtime.GOARCH)
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "sing-box-1.12.17-windows-amd64.zip", nil
		case "arm64":
			return "sing-box-1.12.17-windows-arm64.zip", nil
		case "386":
			return "sing-box-1.12.17-windows-386.zip", nil
		default:
			return "", fmt.Errorf("unsupported windows arch: %s", runtime.GOARCH)
		}
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "sing-box-1.12.17.exe"
	}
	return "sing-box-1.12.17"
}

func downloadReleaseAsset(assetName, destination string) error {
	_, err := runCommandOutput("", "gh", "release", "download", "v1.12.17", "--repo", "sagernet/sing-box", "-p", assetName, "-D", destination)
	return err
}

func extractTarGz(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		targetPath, err := safeJoin(destination, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(targetFile, reader); err != nil {
				targetFile.Close()
				return err
			}
			if err := targetFile.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported tar entry: %s", header.Name)
		}
	}
	return nil
}

func extractZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		targetPath, err := safeJoin(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			fileReader.Close()
			return err
		}
		if _, err := io.Copy(targetFile, fileReader); err != nil {
			fileReader.Close()
			targetFile.Close()
			return err
		}
		fileReader.Close()
		if err := targetFile.Close(); err != nil {
			return err
		}
	}
	return nil
}

func safeJoin(rootDirectory, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid archive path: %s", name)
	}
	return filepath.Join(rootDirectory, cleanName), nil
}

func findBinary(rootDirectory string) (string, error) {
	targetName := "sing-box"
	if runtime.GOOS == "windows" {
		targetName = "sing-box.exe"
	}
	var found string
	walkErr := filepath.WalkDir(rootDirectory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == targetName {
			found = path
			return io.EOF
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, io.EOF) {
		return "", walkErr
	}
	if found == "" {
		return "", errors.New("sing-box binary not found in archive")
	}
	return found, nil
}

func runCommand(t *testing.T, directory, name string, args ...string) {
	t.Helper()
	output, err := runCommandOutput(directory, name, args...)
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
}

func runCommandOutput(directory, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	if directory != "" {
		command.Dir = directory
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return output, err
	}
	return output, nil
}

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileHasData(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 0
}

func ensureDatabase(t *testing.T, cacheDirectory, localPath, repo, assetName string) string {
	t.Helper()
	if fileHasData(localPath) {
		return localPath
	}
	targetPath := filepath.Join(cacheDirectory, assetName)
	if !fileHasData(targetPath) {
		if _, err := exec.LookPath("gh"); err != nil {
			t.Skip("gh not installed")
		}
		if err := downloadReleaseAssetFrom(repo, assetName, cacheDirectory); err != nil {
			t.Fatalf("download %s: %v", assetName, err)
		}
	}
	if !fileHasData(targetPath) {
		t.Fatalf("missing %s after download", assetName)
	}
	return targetPath
}

func downloadReleaseAssetFrom(repo, assetName, destination string) error {
	_, err := runCommandOutput("", "gh", "release", "download", "--repo", repo, "-p", assetName, "-D", destination)
	return err
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func findRepoRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", err
	}
	current := start
	for {
		if fileExists(filepath.Join(current, "go.mod")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errors.New("go.mod not found")
}
