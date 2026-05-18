//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"maps"
	"sync"

	"github.com/sagernet/sing-box/option"
)

// clientAssignment runs in two modes that share active-busid tracking:
// matched (len(targets) > 0) binds each target to at most one busid;
// import-all (len(targets) == 0) marks every advertised busid as desired.
type clientAssignment struct {
	access sync.Mutex

	targets          []clientTarget
	assigned         []string
	matchedKnownKeys map[string]DeviceKey

	allDesired map[string]struct{}
	registered map[string]struct{}

	activeBusIDs map[string]struct{}
}

func newClientAssignment(matches []option.USBIPDeviceMatch) *clientAssignment {
	var targets []clientTarget
	if len(matches) > 0 {
		seenFixed := make(map[string]struct{})
		targets = make([]clientTarget, 0, len(matches))
		for _, m := range matches {
			if m.BusID != "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == "" {
				if _, seen := seenFixed[m.BusID]; seen {
					continue
				}
				seenFixed[m.BusID] = struct{}{}
				targets = append(targets, clientTarget{fixedBusID: m.BusID})
				continue
			}
			targets = append(targets, clientTarget{match: m})
		}
	}
	return &clientAssignment{
		targets:      targets,
		allDesired:   make(map[string]struct{}),
		registered:   make(map[string]struct{}),
		activeBusIDs: make(map[string]struct{}),
	}
}

func (a *clientAssignment) Matched() bool {
	return len(a.targets) > 0
}

func (a *clientAssignment) SetActive(busid string, active bool) {
	a.access.Lock()
	defer a.access.Unlock()
	if active {
		a.activeBusIDs[busid] = struct{}{}
	} else {
		delete(a.activeBusIDs, busid)
	}
}

func (a *clientAssignment) ApplyMatched(entries []DeviceEntry, knownKeys map[string]DeviceKey) (next []string, previous []string) {
	a.access.Lock()
	defer a.access.Unlock()
	if len(a.targets) == 0 {
		return nil, nil
	}
	if a.assigned == nil {
		a.assigned = make([]string, len(a.targets))
	}
	assignmentKeys := a.matchedKeysForAssignmentLocked(entries, knownKeys)
	activeCurrent := a.activeCurrentAssignmentsLocked(a.assigned, assignmentKeys)
	nextAssigned := assignMatchedBusIDsWithRetained(a.targets, a.assigned, entries, assignmentKeys, activeCurrent)
	prev := append([]string(nil), a.assigned...)
	a.assigned = nextAssigned
	a.retainMatchedKnownKeysLocked(assignmentKeys, entries, nextAssigned)
	return nextAssigned, prev
}

// ApplyAll keeps no-longer-desired-but-active busids registered so the
// runBusIDLoop exits naturally after the active session ends, via
// IsRetryDesired returning false.
func (a *clientAssignment) ApplyAll(entries []DeviceEntry) (start []string, stop []string) {
	desired := make(map[string]struct{}, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		desired[busid] = struct{}{}
	}
	a.access.Lock()
	defer a.access.Unlock()
	a.allDesired = desired
	for busid := range a.registered {
		if _, ok := desired[busid]; ok {
			continue
		}
		if _, active := a.activeBusIDs[busid]; active {
			continue
		}
		stop = append(stop, busid)
		delete(a.registered, busid)
	}
	for busid := range desired {
		if _, ok := a.registered[busid]; ok {
			continue
		}
		start = append(start, busid)
		a.registered[busid] = struct{}{}
	}
	return start, stop
}

func (a *clientAssignment) IsRetryDesired(busid string) bool {
	a.access.Lock()
	defer a.access.Unlock()
	if _, registered := a.registered[busid]; !registered {
		return false
	}
	_, desired := a.allDesired[busid]
	return desired
}

func (a *clientAssignment) matchedKeysForAssignmentLocked(entries []DeviceEntry, knownKeys map[string]DeviceKey) map[string]DeviceKey {
	if len(a.matchedKnownKeys) == 0 && len(entries) == 0 && len(knownKeys) == 0 {
		return nil
	}
	assignmentKeys := make(map[string]DeviceKey, len(a.matchedKnownKeys)+len(entries)+len(knownKeys))
	maps.Copy(assignmentKeys, a.matchedKnownKeys)
	for i := range entries {
		key := entryDeviceKey(entries[i])
		if key.BusID == "" {
			continue
		}
		assignmentKeys[key.BusID] = key
	}
	for busid, key := range knownKeys {
		if busid == "" {
			continue
		}
		assignmentKeys[busid] = key
	}
	return assignmentKeys
}

func (a *clientAssignment) retainMatchedKnownKeysLocked(assignmentKeys map[string]DeviceKey, entries []DeviceEntry, assigned []string) {
	if len(assignmentKeys) == 0 {
		a.matchedKnownKeys = nil
		return
	}
	retained := make(map[string]DeviceKey, len(entries)+len(assigned))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		if key, ok := assignmentKeys[busid]; ok {
			retained[busid] = key
		}
	}
	for _, busid := range assigned {
		if busid == "" {
			continue
		}
		if key, ok := assignmentKeys[busid]; ok {
			retained[busid] = key
		}
	}
	if len(retained) == 0 {
		a.matchedKnownKeys = nil
		return
	}
	a.matchedKnownKeys = retained
}

func assignMatchedBusIDsWithRetained(
	targets []clientTarget,
	current []string,
	entries []DeviceEntry,
	knownKeys map[string]DeviceKey,
	activeCurrent map[string]struct{},
) []string {
	if len(targets) == 0 {
		return nil
	}
	keysByBusID := make(map[string]DeviceKey, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		keysByBusID[busid] = entryDeviceKey(entries[i])
	}
	currentKey := func(busid string) (DeviceKey, bool) {
		if key, ok := keysByBusID[busid]; ok {
			return key, true
		}
		if _, active := activeCurrent[busid]; !active {
			return DeviceKey{}, false
		}
		key, ok := knownKeys[busid]
		return key, ok
	}
	nextAssigned := make([]string, len(targets))
	reserved := make(map[string]struct{}, len(targets))
	for i, target := range targets {
		if target.fixedBusID == "" {
			continue
		}
		if _, ok := keysByBusID[target.fixedBusID]; ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
			continue
		}
		if i >= len(current) || current[i] != target.fixedBusID {
			continue
		}
		if _, ok := currentKey(target.fixedBusID); ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
		}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || i >= len(current) || current[i] == "" {
			continue
		}
		if _, ok := reserved[current[i]]; ok {
			continue
		}
		key, ok := currentKey(current[i])
		if !ok || !matches(target.match, key) {
			continue
		}
		nextAssigned[i] = current[i]
		reserved[current[i]] = struct{}{}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || nextAssigned[i] != "" {
			continue
		}
		for j := range entries {
			key := entryDeviceKey(entries[j])
			if _, claimed := reserved[key.BusID]; claimed {
				continue
			}
			if matches(target.match, key) {
				nextAssigned[i] = key.BusID
				reserved[key.BusID] = struct{}{}
				break
			}
		}
	}
	return nextAssigned
}

func (a *clientAssignment) activeCurrentAssignmentsLocked(current []string, knownKeys map[string]DeviceKey) map[string]struct{} {
	if len(knownKeys) == 0 {
		return nil
	}
	var activeCurrent map[string]struct{}
	for _, busid := range current {
		if busid == "" {
			continue
		}
		if _, ok := knownKeys[busid]; !ok {
			continue
		}
		if _, active := a.activeBusIDs[busid]; !active {
			continue
		}
		if activeCurrent == nil {
			activeCurrent = make(map[string]struct{})
		}
		activeCurrent[busid] = struct{}{}
	}
	return activeCurrent
}
