package pipe

import (
	"github.com/sagernet/sing-box/common/xray/signal"
	"github.com/sagernet/sing-box/common/xray/signal/done"
)

// Option for creating new Pipes.
type Option func(*pipeOption)

// WithoutSizeLimit returns an Option for Pipe to have no size limit.
func WithoutSizeLimit() Option {
	return func(opt *pipeOption) {
		opt.limit = -1
	}
}

// WithSizeLimit returns an Option for Pipe to have the given size limit.
func WithSizeLimit(limit int32) Option {
	return func(opt *pipeOption) {
		opt.limit = limit
	}
}

// DiscardOverflow returns an Option for Pipe to discard writes if full.
func DiscardOverflow() Option {
	return func(opt *pipeOption) {
		opt.discardOverflow = true
	}
}

// New creates a new Reader and Writer that connects to each other.
func New(opts ...Option) (*Reader, *Writer) {
	p := &pipe{
		readSignal:  signal.NewNotifier(),
		writeSignal: signal.NewNotifier(),
		done:        done.New(),
		errChan:     make(chan error, 1),
		option: pipeOption{
			limit: -1,
		},
	}

	for _, opt := range opts {
		opt(&(p.option))
	}

	return &Reader{
			pipe: p,
		}, &Writer{
			pipe: p,
		}
}
