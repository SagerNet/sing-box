package limiter

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/sagernet/sing-box/common/humanize"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

const (
	limiterTag     = "tag"
	limiterUser    = "user"
	limiterInbound = "inbound"
)

var _ Manager = (*defaultManager)(nil)

type defaultManager struct {
	mp *sync.Map
}

func WithDefault(ctx context.Context, logger log.ContextLogger, options []option.Limiter) context.Context {
	m := &defaultManager{mp: &sync.Map{}}
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

func buildKey(prefix string, tag string) string {
	return fmt.Sprintf("%s|%s", prefix, tag)
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
		m.mp.Store(buildKey(limiterTag, option.Tag), l)
	}
	if len(option.AuthUser) > 0 {
		valid = true
		for _, user := range option.AuthUser {
			m.mp.Store(buildKey(limiterUser, user), l)
		}
	}
	if len(option.Inbound) > 0 {
		valid = true
		for _, inbound := range option.Inbound {
			m.mp.Store(buildKey(limiterInbound, inbound), l)
		}
	}
	if !valid {
		return E.New("tag/user/inbound, at least one must be set")
	}
	return
}

func (m *defaultManager) LoadLimiters(tags []string, user, inbound string) (limiters []*limiter) {
	for _, t := range tags {
		if v, ok := m.mp.Load(buildKey(limiterTag, t)); ok {
			limiters = append(limiters, v.(*limiter))
		}
	}
	if v, ok := m.mp.Load(buildKey(limiterUser, user)); ok {
		limiters = append(limiters, v.(*limiter))
	}
	if v, ok := m.mp.Load(buildKey(limiterInbound, inbound)); ok {
		limiters = append(limiters, v.(*limiter))
	}
	return
}

func (m *defaultManager) NewConnWithLimiters(ctx context.Context, conn net.Conn, limiters []*limiter) net.Conn {
	for _, limiter := range limiters {
		conn = &connWithLimiter{Conn: conn, limiter: limiter, ctx: ctx}
	}
	return conn
}
