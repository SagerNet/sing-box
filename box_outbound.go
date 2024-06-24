package box

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func (s *Box) startOutbounds() error {
	monitor := taskmonitor.New(s.logger, C.StartTimeout)
	outboundTags := make(map[adapter.Outbound]string)
	outbounds := make(map[string]adapter.Outbound)
	for i, outboundToStart := range s.outbounds {
		var outboundTag string
		if outboundToStart.Tag() == "" {
			outboundTag = F.ToString(i)
		} else {
			outboundTag = outboundToStart.Tag()
		}
		if _, exists := outbounds[outboundTag]; exists {
			return E.New("outbound tag ", outboundTag, " duplicated")
		}
		outboundTags[outboundToStart] = outboundTag
		outbounds[outboundTag] = outboundToStart
	}
	started := make(map[string]bool)
	for {
		canContinue := false
	startOne:
		for _, outboundToStart := range s.outbounds {
			outboundTag := outboundTags[outboundToStart]
			if started[outboundTag] {
				continue
			}
			dependencies := outboundToStart.Dependencies()
			for _, dependency := range dependencies {
				if !started[dependency] {
					continue startOne
				}
			}
			started[outboundTag] = true
			canContinue = true
			if starter, isStarter := outboundToStart.(interface {
				Start() error
			}); isStarter {
				monitor.Start("initialize outbound/", outboundToStart.Type(), "[", outboundTag, "]")
				err := starter.Start()
				monitor.Finish()
				if err != nil {
					return E.Cause(err, "initialize outbound/", outboundToStart.Type(), "[", outboundTag, "]")
				}
			}
		}
		if len(started) == len(s.outbounds) {
			break
		}
		if canContinue {
			continue
		}
		currentOutbound := common.Find(s.outbounds, func(it adapter.Outbound) bool {
			return !started[outboundTags[it]]
		})
		var lintOutbound func(oTree []string, oCurrent adapter.Outbound) error
		lintOutbound = func(oTree []string, oCurrent adapter.Outbound) error {
			problemOutboundTag := common.Find(oCurrent.Dependencies(), func(it string) bool {
				return !started[it]
			})
			if common.Contains(oTree, problemOutboundTag) {
				return E.New("circular outbound dependency: ", strings.Join(oTree, " -> "), " -> ", problemOutboundTag)
			}
			problemOutbound := outbounds[problemOutboundTag]
			if problemOutbound == nil {
				return E.New("dependency[", problemOutboundTag, "] not found for outbound[", outboundTags[oCurrent], "]")
			}
			return lintOutbound(append(oTree, problemOutboundTag), problemOutbound)
		}
		return lintOutbound([]string{outboundTags[currentOutbound]}, currentOutbound)
	}
	return nil
}
