package daemon

import (
	"github.com/sagernet/sing-box/log"
)

type logRing struct {
	entries  []*log.Entry
	maxLines int
	start    int
}

func (r *logRing) push(entry *log.Entry) {
	if r.maxLines <= 0 {
		return
	}
	if len(r.entries) < r.maxLines {
		r.entries = append(r.entries, entry)
		return
	}
	r.entries[r.start] = entry
	r.start++
	if r.start == len(r.entries) {
		r.start = 0
	}
}

func (r *logRing) array() []*log.Entry {
	result := make([]*log.Entry, 0, len(r.entries))
	result = append(result, r.entries[r.start:]...)
	result = append(result, r.entries[:r.start]...)
	return result
}

func (r *logRing) reset() {
	clear(r.entries)
	r.entries = r.entries[:0]
	r.start = 0
}
