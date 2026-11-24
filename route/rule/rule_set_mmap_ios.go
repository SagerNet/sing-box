//go:build ios

package rule

import (
	"context"
	"io"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service/filemanager"
)

func mmapRuleSet(ctx context.Context, logger logger.Logger, tag string, ruleSet option.PlainRuleSetCompat) (option.PlainRuleSetCompat, error) {
	file, err := filemanager.CreateTemp(ctx, "rule-set-*.mmap")
	if err != nil {
		logger.Warn(E.Cause(err, "create rule-set mmap file ", tag))
		return ruleSet, nil
	}
	defer file.Close()
	err = srs.WriteMmap(file, ruleSet)
	if err != nil {
		filemanager.Remove(ctx, file.Name())
		logger.Warn(E.Cause(err, "write rule-set mmap file ", tag))
		return ruleSet, nil
	}
	ruleSet = option.PlainRuleSetCompat{}
	_, err = file.Seek(0, io.SeekStart)
	if err == nil {
		ruleSet, err = srs.ReadMmap(file)
	}
	if err != nil {
		filemanager.Remove(ctx, file.Name())
		return option.PlainRuleSetCompat{}, E.Cause(err, "mmap rule-set ", tag)
	}
	logger.Debug("mmap rule-set ", tag)
	return ruleSet, nil
}
