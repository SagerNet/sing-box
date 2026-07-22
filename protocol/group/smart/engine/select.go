package engine

import (
	"hash/fnv"
	"math"
	"sort"
	"time"
)

// Candidate is an ordered dial attempt target.
type Candidate struct {
	Tag   string
	Score float64
}

type scoredMember struct {
	tag      string
	score    float64
	base     float64
	attempts int
}

// Select returns ordered members to try for a dial (host may be empty).
// Order: preferred (API) -> host sticky -> contextual bandit cost.
func (e *Engine) Select(host string) []Candidate {
	return e.SelectFor(host, NetworkTCP)
}

// SelectFor combines protocol-wide health with bounded target-specific history.
func (e *Engine) SelectFor(host string, network Network) []Candidate {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.opts.Now()
	members := e.memberMapLocked(network)
	key := targetKey(host, network)
	target := e.targets[key]
	if target != nil {
		target.UpdatedAt = now
	}

	rows := make([]scoredMember, 0, len(e.order))
	totalAttempts := 0
	for _, tag := range e.order {
		m := members[tag]
		if m == nil {
			continue
		}
		e.decayPenaltyLocked(m, now)
		score := e.baseCostLocked(m, now)
		attempts := m.Attempts + m.FirstByteAttempts
		if target != nil {
			if local := target.Members[tag]; local != nil && local.Attempts+local.FirstByteAttempts > 0 {
				e.decayPenaltyLocked(local, now)
				localAttempts := local.Attempts + local.FirstByteAttempts
				confidence := math.Min(0.75, float64(localAttempts)/8*0.75)
				score = score*(1-confidence) + e.baseCostLocked(local, now)*confidence
				attempts = localAttempts
			} else {
				attempts = 0
			}
		}
		if attempts < 0 {
			attempts = 0
		}
		totalAttempts += attempts
		rows = append(rows, scoredMember{tag: tag, score: score, base: score, attempts: attempts})
	}
	if len(rows) == 0 {
		return nil
	}
	// Lower confidence bound for a cost metric. The finite bonus explores arms
	// with less evidence and naturally fades as contextual samples accumulate.
	for i := range rows {
		bonus := e.opts.ExplorationMs * math.Sqrt(
			math.Log(float64(totalAttempts)+2)/float64(rows[i].attempts+1),
		)
		rows[i].score -= math.Min(e.opts.ExplorationMs*2, bonus)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score < rows[j].score })

	// Host sticky wins over Top-K/explore when still competitive.
	if host != "" {
		if sticky, ok := e.hosts[key]; ok && now.Sub(sticky.UpdatedAt) <= e.opts.HostStickyTTL {
			best := rows[0].base
			for i, r := range rows {
				if r.tag != sticky.Tag {
					continue
				}
				if i == 0 || r.base <= best*2+50 {
					if i > 0 {
						rows = append([]scoredMember{r}, append(rows[:i], rows[i+1:]...)...)
					}
				}
				break
			}
		}
	}

	// Preferred (app/API) wins while it remains competitive. A destination that
	// repeatedly fails may use its learned fallback without changing group now.
	if e.preferred != "" {
		best := rows[0].base
		for i, r := range rows {
			if r.tag != e.preferred {
				continue
			}
			if i > 0 && r.base <= best*2+50 {
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
	e.RememberHostFor(host, NetworkTCP, tag)
}

func (e *Engine) RememberHostFor(host string, network Network, tag string) {
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
	e.hosts[targetKey(host, network)] = hostSticky{Tag: tag, UpdatedAt: e.opts.Now()}
}

func (e *Engine) baseCostLocked(m *MemberStats, now time.Time) float64 {
	base := m.EwmaMs
	if m.Samples == 0 {
		if m.HasURLTestPrior {
			base = float64(m.URLTestLatencyMs)
		} else {
			// Cold default + deterministic jitter so listed order is not always first.
			base = 800 + float64(tagJitter(m.Tag)%200)
		}
	}
	firstByte := m.FirstByteEwmaMs
	if m.FirstByteSamples == 0 {
		firstByte = base * 1.5
	}
	base = base*e.opts.RTTWeight +
		firstByte*e.opts.FirstByteWeight +
		m.JitterMs*e.opts.JitterWeight
	base += m.FailureRate * 600
	base += m.FirstByteFailRate * 500
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
