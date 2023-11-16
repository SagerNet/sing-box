package limiter

import (
	"context"
	"net"

	"golang.org/x/time/rate"
)

type limiter struct {
	downloadLimiter *rate.Limiter
	uploadLimiter   *rate.Limiter
}

func newLimiter(download, upload uint64) *limiter {
	var downloadLimiter, uploadLimiter *rate.Limiter
	if download > 0 {
		downloadLimiter = rate.NewLimiter(rate.Limit(float64(download)), int(download))
	}
	if upload > 0 {
		uploadLimiter = rate.NewLimiter(rate.Limit(float64(upload)), int(upload))
	}
	return &limiter{downloadLimiter: downloadLimiter, uploadLimiter: uploadLimiter}
}

type connWithLimiter struct {
	net.Conn
	limiter *limiter
	ctx     context.Context
}

func (conn *connWithLimiter) Read(p []byte) (n int, err error) {
	if conn.limiter == nil || conn.limiter.uploadLimiter == nil {
		return conn.Conn.Read(p)
	}
	b := conn.limiter.uploadLimiter.Burst()
	if b < len(p) {
		p = p[:b]
	}
	n, err = conn.Conn.Read(p)
	if err != nil {
		return
	}
	err = conn.limiter.uploadLimiter.WaitN(conn.ctx, n)
	if err != nil {
		return
	}
	return
}

func (conn *connWithLimiter) Write(p []byte) (n int, err error) {
	if conn.limiter == nil || conn.limiter.downloadLimiter == nil {
		return conn.Conn.Write(p)
	}
	var nn int
	b := conn.limiter.downloadLimiter.Burst()
	for {
		end := len(p)
		if end == 0 {
			break
		}
		if b < len(p) {
			end = b
		}
		err = conn.limiter.downloadLimiter.WaitN(conn.ctx, end)
		if err != nil {
			return
		}
		nn, err = conn.Conn.Write(p[:end])
		n += nn
		if err != nil {
			return
		}
		p = p[end:]
	}
	return
}
