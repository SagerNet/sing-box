package limiter

import (
	"context"
	"fmt"
	"net"

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
	if len(option.Download) > 0 {
		download, err = humanize.ParseBytes(option.Download)
		if err != nil {
			return err
		}
	}
	if len(option.Upload) > 0 {
		upload, err = humanize.ParseBytes(option.Upload)
		if err != nil {
			return err
		}
	}
	if download == 0 && upload == 0 {
		return E.New("download/upload, at least one must be set")
	}
	l := newLimiter(download, upload)
	valid := false
	if len(option.Tag) > 0 {
		valid = true
		m.mp[limiterKey{prefixTag, option.Tag}] = l
	}
	if len(option.AuthUser) > 0 {
		valid = true
		for _, user := range option.AuthUser {
			m.mp[limiterKey{prefixUser, user}] = l
		}
	}
	if len(option.Inbound) > 0 {
		valid = true
		for _, inbound := range option.Inbound {
			m.mp[limiterKey{prefixInbound, inbound}] = l
		}
	}
	if !valid {
		return E.New("tag/user/inbound, at least one must be set")
	}
	return
}

func (m *defaultManager) LoadLimiters(tags []string, user, inbound string) (limiters []*limiter) {
	for _, tag := range tags {
		if v, ok := m.mp[limiterKey{prefixTag, tag}]; ok {
			limiters = append(limiters, v)
		}
	}
	if v, ok := m.mp[limiterKey{prefixUser, user}]; ok {
		limiters = append(limiters, v)
	}
	if v, ok := m.mp[limiterKey{prefixInbound, inbound}]; ok {
		limiters = append(limiters, v)
	}
	return
}

func (m *defaultManager) NewConnWithLimiters(ctx context.Context, conn net.Conn, limiters []*limiter) net.Conn {
	for _, limiter := range limiters {
		conn = &connWithLimiter{Conn: conn, limiter: limiter, ctx: ctx}
	}
	return conn
}
