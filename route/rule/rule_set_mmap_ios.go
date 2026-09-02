//go:build ios

package rule

import (
	"context"
	"io"
	"os"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service/filemanager"
)

func mmapRuleSet(ctx context.Context, logger logger.Logger, tag string, ruleSet option.PlainRuleSetCompat) option.PlainRuleSetCompat {
	file, err := filemanager.CreateTemp(ctx, "rule-set-*.mmap")
	if err != nil {
		logger.Warn(E.Cause(err, "create rule-set mmap file ", tag))
		return ruleSet
	}
	defer file.Close()
	mmap, err := writeAndMmapRuleSet(file, ruleSet)
	if err != nil {
		filemanager.Remove(ctx, file.Name())
		logger.Warn(E.Cause(err, "mmap rule-set ", tag))
		return ruleSet
	}
	logger.Debug("mmap rule-set ", tag)
	return mmap
}

func writeAndMmapRuleSet(file *os.File, ruleSet option.PlainRuleSetCompat) (option.PlainRuleSetCompat, error) {
	err := srs.WriteMmap(file, ruleSet)
	if err != nil {
		return ruleSet, err
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return ruleSet, err
	}
	return srs.ReadMmap(file)
}
