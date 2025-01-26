package tf

import (
	"bufio"
	"bytes"
	_ "embed"
	"io"
	"strings"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/domain"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var publicPrefix = []string{
	"www",
}

//go:generate wget -O public_suffix_list.dat https://publicsuffix.org/list/public_suffix_list.dat

//go:embed public_suffix_list.dat
var publicSuffix []byte

var publicSuffixMatcher = common.OnceValue(func() *domain.Matcher {
	matcher, err := initPublicSuffixMatcher()
	if err != nil {
		panic(F.ToString("error in initialize public suffix matcher"))
	}
	return matcher
})

func initPublicSuffixMatcher() (*domain.Matcher, error) {
	reader := bufio.NewReader(bytes.NewReader(publicSuffix))
	var domainList []string
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if isPrefix {
			return nil, E.New("unexpected prefix line")
		}
		lineStr := string(line)
		lineStr = strings.TrimSpace(lineStr)
		if lineStr == "" || strings.HasPrefix(lineStr, "//") {
			continue
		}
		domainList = append(domainList, lineStr)
	}
	return domain.NewMatcher(domainList, nil, false), nil
}
