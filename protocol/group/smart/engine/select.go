package engine

import (
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"time"
)

// Candidate is an ordered dial attempt target.
type Candidate struct {
	Tag   string
	Score float64
}

type scoredMember struct {
	tag   string
	score float64
}

// Select returns ordered members to try for a dial (host may be empty).
// Order: preferred (API) -> host sticky -> top-K score (+ exploration) -> rest by score.
func (e *Engine) Select(host string) []Candidate {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.opts.Now()

	rows := make([]scoredMember, 0, len(e.order))
	for _, tag := range e.order {
		m := e.members[tag]
		if m == nil {
			continue
		}
		e.decayPenaltyLocked(m, now)
		rows = append(rows, scoredMember{tag: tag, score: e.scoreLocked(m, now)})
	}
	if len(rows) == 0 {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].score < rows[j].score
	})

	// Top-K weighted random among the best K (before sticky/preferred pins).
	k := e.opts.TopK
	if k > len(rows) {
		k = len(rows)
	}
	if k > 1 {
		scores := make([]float64, k)
		for i := 0; i < k; i++ {
			scores[i] = rows[i].score
		}
		pick := softmaxPickIndex(scores, now.UnixNano())
		if pick > 0 && pick < k {
			chosen := rows[pick]
			copy(rows[1:pick+1], rows[0:pick])
			rows[0] = chosen
		}
	}

	// Exploration: sometimes try a cold / non-head member first (before sticky).
	if e.opts.ExploreProb > 0 && len(rows) > 1 {
		r := rand.New(rand.NewSource(now.UnixNano() + int64(len(rows))*9973))
		if r.Float64() < e.opts.ExploreProb {
			cold := make([]int, 0, len(rows))
			for i := 1; i < len(rows); i++ {
				m := e.members[rows[i].tag]
				if m != nil && m.Samples == 0 {
					cold = append(cold, i)
				}
			}
			idx := 1 + r.Intn(len(rows)-1)
			if len(cold) > 0 {
				idx = cold[r.Intn(len(cold))]
			}
			if idx > 0 && idx < len(rows) {
				chosen := rows[idx]
				copy(rows[1:idx+1], rows[0:idx])
				rows[0] = chosen
			}
		}
	}

	// Host sticky wins over Top-K/explore when still competitive.
	if host != "" {
		if sticky, ok := e.hosts[host]; ok && now.Sub(sticky.UpdatedAt) <= e.opts.HostStickyTTL {
			best := rows[0].score
			for i, r := range rows {
				if r.tag != sticky.Tag {
					continue
				}
				if i == 0 || r.score <= best*2+50 {
					if i > 0 {
						rows = append([]scoredMember{r}, append(rows[:i], rows[i+1:]...)...)
					}
				}
				break
			}
		}
	}

	// Preferred (app/API) wins over sticky for the next dials.
	if e.preferred != "" {
		for i, r := range rows {
			if r.tag != e.preferred {
				continue
			}
			if i > 0 {
				rows = append([]scoredMember{r}, append(rows[:i], rows[i+1:]...)...)
			}
			break
		}
	}

	out := make([]Candidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, Candidate{Tag: r.tag, Score: r.score})
	}
	return out
}

// RememberHost records sticky association after a successful dial to host.
func (e *Engine) RememberHost(host, tag string) {
	if host == "" || tag == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.hosts) >= e.opts.MaxHosts {
		var oldest string
		var oldestT time.Time
		first := true
		for h, st := range e.hosts {
			if first || st.UpdatedAt.Before(oldestT) {
				first = false
				oldest = h
				oldestT = st.UpdatedAt
			}
		}
		delete(e.hosts, oldest)
	}
	e.hosts[host] = hostSticky{Tag: tag, UpdatedAt: e.opts.Now()}
}

func (e *Engine) scoreLocked(m *MemberStats, now time.Time) float64 {
	base := m.EwmaMs
	if m.Samples == 0 {
		if m.HasURLTestPrior {
			base = float64(m.URLTestLatencyMs)
		} else {
			// Cold default + deterministic jitter so listed order is not always first.
			base = 800 + float64(tagJitter(m.Tag)%200)
		}
	}
	base += m.JitterMs * 0.5
	base += m.FailureRate * 600
	base += float64(m.ConsecutiveFails) * 180
	base += m.Penalty * 100
	if m.Weight > 0 {
		base *= m.Weight
	}
	if !m.LastSuccess.IsZero() {
		stale := now.Sub(m.LastSuccess)
		if stale > 3*time.Minute {
			base += float64(stale/time.Minute) * 5
		}
	}
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return 1e9
	}
	return base
}

func tagJitter(tag string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tag))
	return h.Sum32()
}

func softmaxPickIndex(scores []float64, seed int64) int {
	if len(scores) <= 1 {
		return 0
	}
	const temp = 40.0
	maxNeg := -scores[0] / temp
	for _, s := range scores {
		v := -s / temp
		if v > maxNeg {
			maxNeg = v
		}
	}
	sum := 0.0
	weights := make([]float64, len(scores))
	for i, s := range scores {
		w := math.Exp(-s/temp - maxNeg)
		weights[i] = w
		sum += w
	}
	if sum <= 0 {
		return 0
	}
	r := rand.New(rand.NewSource(seed))
	x := r.Float64() * sum
	acc := 0.0
	for i, w := range weights {
		acc += w
		if x <= acc {
			return i
		}
	}
	return len(scores) - 1
}
