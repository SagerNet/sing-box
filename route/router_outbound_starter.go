package route

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	E "github.com/sagernet/sing/common/exceptions"
)

type OutboundStarter struct {
	outboundByTag map[string]adapter.Outbound
	startedTags   map[string]struct{}
	monitor       *taskmonitor.Monitor
}

func (s *OutboundStarter) Start(tag string, pathIncludesTags map[string]struct{}) error {
	adapter := s.outboundByTag[tag]
	if adapter == nil {
		return E.New("dependency[", tag, "] is not found")
	}

	// The outbound may have been started by another subtree in the previous,
	// we don't need to start it again.
	if _, ok := s.startedTags[tag]; ok {
		return nil
	}

	// If we detected the repetition of the tags in scope of tree evaluation,
	// the circular dependency is found, as it grows from bottom to top.
	if _, ok := pathIncludesTags[tag]; ok {
		return E.New("circular dependency related with outbound/", adapter.Type(), "[", tag, "]")
	}

	// This required to be done only if that outbound isn't already started,
	// because some dependencies may come to the same root,
	// but they aren't circular.
	pathIncludesTags[tag] = struct{}{}

	// Next, we are recursively starting all dependencies of the current
	// outbound and repeating the cycle.
	for _, dependencyTag := range adapter.Dependencies() {
		if err := s.Start(dependencyTag, pathIncludesTags); err != nil {
			return err
		}
	}

	// Anyway, it will be finished soon, nothing will happen if I'll include
	// Startable interface typecasting too.
	s.monitor.Start("initialize outbound/", adapter.Type(), "[", tag, "]")
	defer s.monitor.Finish()

	// After the evaluation of entire tree let's begin to start all
	// the outbounds!
	if startable, isStartable := adapter.(interface{ Start() error }); isStartable {
		if err := startable.Start(); err != nil {
			return E.Cause(err, "initialize outbound/", adapter.Type(), "[", tag, "]")
		}
	}

	return nil
}
