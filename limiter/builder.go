package limiter

import (
	"context"
	"fmt"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/humanize"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

const (
	prefixTag     = "tag"
	prefixUser    = "user"
	prefixInbound = "inbound"
)

var _ Manager = (*defaultManager)(nil)

type limiterKey struct {
	Prefix string
	Name   string
}

type defaultManager struct {
	mp map[limiterKey]*limiter
}

func WithDefault(ctx context.Context, logger log.ContextLogger, options []option.Limiter) context.Context {
	m := &defaultManager{mp: make(map[limiterKey]*limiter)}
	for i, option := range options {
		if err := m.createLimiter(ctx, option); err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("id=%d, %s", i, err))
		} else {
			logger.InfoContext(ctx, fmt.Sprintf("id=%d, tag=%s, users=%v, inbounds=%v, download=%s, upload=%s",
				i, option.Tag, option.AuthUser, option.Inbound, option.Download, option.Upload))
		}
	}
	return service.ContextWith[Manager](ctx, m)
}

func (m *defaultManager) createLimiter(ctx context.Context, option option.Limiter) (err error) {
	var download, upload uint64
	if option.Download != "" {
		download, err = humanize.ParseBytes(option.Download)
		if err != nil {
			return err
		}
	}
	if option.Upload != "" {
		upload, err = humanize.ParseBytes(option.Upload)
		if err != nil {
			return err
		}
	}
	if download == 0 && upload == 0 {
		return E.New("download/upload, at least one must be set")
	}
	if option.Tag == "" && len(option.AuthUser) == 0 && len(option.Inbound) == 0 {
		return E.New("tag/user/inbound, at least one must be set")
	}
	var sharedLimiter *limiter
	if option.Tag != "" || !option.AuthUserIndependent || !option.InboundIndependent {
		sharedLimiter = newLimiter(download, upload)
	}
	if option.Tag != "" {
		m.mp[limiterKey{prefixTag, option.Tag}] = sharedLimiter
	}
	for _, user := range option.AuthUser {
		if option.AuthUserIndependent {
			m.mp[limiterKey{prefixUser, user}] = newLimiter(download, upload)
		} else {
			m.mp[limiterKey{prefixUser, user}] = sharedLimiter
		}
	}
	for _, inbound := range option.Inbound {
		if option.InboundIndependent {
			m.mp[limiterKey{prefixInbound, inbound}] = newLimiter(download, upload)
		} else {
			m.mp[limiterKey{prefixInbound, inbound}] = sharedLimiter
		}
	}
	return
}

func (m *defaultManager) NewConnWithLimiters(ctx context.Context, conn net.Conn, metadata *adapter.InboundContext, rule adapter.Rule) net.Conn {
	var limiters []*limiter
	if rule != nil {
		for _, tag := range rule.Limiters() {
			if v, ok := m.mp[limiterKey{prefixTag, tag}]; ok {
				limiters = append(limiters, v)
			}
		}
	}
	if metadata != nil {
		if v, ok := m.mp[limiterKey{prefixUser, metadata.User}]; ok {
			limiters = append(limiters, v)
		}
		if v, ok := m.mp[limiterKey{prefixInbound, metadata.Inbound}]; ok {
			limiters = append(limiters, v)
		}
	}
	for _, limiter := range limiters {
		conn = &connWithLimiter{Conn: conn, limiter: limiter, ctx: ctx}
	}
	return conn
}
