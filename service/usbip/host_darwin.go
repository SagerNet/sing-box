//go:build darwin && cgo

package usbip

import (
	"context"
	"fmt"
	"maps"
	"net"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func newPlatformExportHost(ctx context.Context, logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newDarwinExportHost(ctx, logger, matches), nil
}

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return &darwinImportHost{logger: logger}, nil
}

// darwinExportHost retains stale captures: devices that reconcile
// wanted to drop while still busy stay owned until the import ends.
type darwinExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch

	runCtx    context.Context
	runCancel context.CancelFunc

	access  sync.Mutex
	exports map[string]*darwinExport
	watcher *darwinUSBHostDeviceWatcher
}

func newDarwinExportHost(ctx context.Context, logger log.ContextLogger, matches []option.USBIPDeviceMatch) *darwinExportHost {
	runCtx, runCancel := context.WithCancel(ctx)
	return &darwinExportHost{
		runCtx:    runCtx,
		runCancel: runCancel,
		logger:    logger,
		matches:   matches,
		exports:   make(map[string]*darwinExport),
	}
}

func (h *darwinExportHost) Start() error { return nil }

func (h *darwinExportHost) Close() error {
	h.runCancel()
	h.access.Lock()
	watcher := h.watcher
	h.watcher = nil
	exports := h.exports
	h.exports = make(map[string]*darwinExport)
	h.access.Unlock()
	if watcher != nil {
		watcher.Close()
	}
	for _, exp := range exports {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return nil
}

func (h *darwinExportHost) Events() (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	signal := func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	watcher, err := darwinWatchUSBHostDevices(signal)
	if err != nil {
		close(ch)
		return nil, err
	}
	h.access.Lock()
	h.watcher = watcher
	h.access.Unlock()
	go func() {
		<-h.runCtx.Done()
		h.access.Lock()
		w := h.watcher
		h.watcher = nil
		h.access.Unlock()
		if w != nil {
			w.Close()
		}
		close(ch)
	}()
	return ch, nil
}

func (h *darwinExportHost) Reconcile(isReserved func(busid string) bool) (map[string]Export, []string, error) {
	devices, err := darwinCopyUSBHostDevices()
	if err != nil {
		return h.snapshotSelf(), nil, E.Cause(err, "enumerate IOUSBHost devices")
	}
	keys := make([]DeviceKey, len(devices))
	for i := range devices {
		keys[i] = devices[i].key
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, idx := range SelectMatches(h.matches, keys) {
		if devices[idx].entry.Info.BDeviceClass == 0x09 {
			h.logger.Warn("skip hub device ", devices[idx].key.BusID)
			continue
		}
		desired[devices[idx].key.BusID] = devices[idx]
	}

	h.access.Lock()
	current := make(map[string]*darwinExport, len(h.exports))
	maps.Copy(current, h.exports)
	h.access.Unlock()

	var (
		toAdd    []*darwinExport
		toRemove []*darwinExport
		toStale  []darwinStaleMark
		released []string
	)
	for busid, info := range desired {
		if exp, ok := current[busid]; ok && exp.registryID == info.registryID {
			continue
		}
		if exp, ok := current[busid]; ok {
			if isReserved(busid) {
				toStale = append(toStale, darwinStaleMark{busid: busid, pendingRegistryID: info.registryID})
				continue
			}
			toRemove = append(toRemove, exp)
			released = append(released, busid)
		}
		device, err := darwinOpenUSBHostDevice(info.registryID, true)
		if err != nil {
			h.logger.Warn("capture ", busid, ": ", err)
			continue
		}
		info = device.info
		toAdd = append(toAdd, &darwinExport{
			busid:      info.key.BusID,
			registryID: info.registryID,
			device:     device,
			entry:      info.entry,
			logger:     h.logger,
		})
		h.logger.Info("exported ", info.key.BusID, " through IOUSBHost capture")
	}
	for busid, exp := range current {
		if _, ok := desired[busid]; ok {
			continue
		}
		if isReserved(busid) {
			toStale = append(toStale, darwinStaleMark{busid: busid})
			continue
		}
		toRemove = append(toRemove, exp)
		h.logger.Info("released ", busid, " from IOUSBHost capture")
		released = append(released, busid)
	}

	committed := make(map[string]*darwinExport, len(current)+len(toAdd))
	maps.Copy(committed, current)
	for _, mark := range toStale {
		exp, found := committed[mark.busid]
		if !found {
			continue
		}
		cloned := cloneDarwinExport(exp)
		cloned.stale = true
		if mark.pendingRegistryID != 0 {
			cloned.pendingRegistryID = mark.pendingRegistryID
		}
		committed[mark.busid] = cloned
	}
	for _, exp := range toRemove {
		delete(committed, exp.busid)
	}
	for _, exp := range toAdd {
		committed[exp.busid] = exp
	}

	h.access.Lock()
	h.exports = committed
	h.access.Unlock()

	for _, exp := range toRemove {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return snapshotDarwinExports(committed), released, nil
}

func (h *darwinExportHost) FinishImport(busid string) (bool, error) {
	h.access.Lock()
	exp, ok := h.exports[busid]
	if !ok || !exp.stale {
		h.access.Unlock()
		return false, nil
	}
	pending := exp.pendingRegistryID
	if pending == 0 {
		delete(h.exports, busid)
		h.access.Unlock()
		if exp.device != nil {
			exp.device.Close()
		}
		return true, nil
	}
	h.access.Unlock()

	device, err := darwinOpenUSBHostDevice(pending, true)
	if err != nil {
		h.logger.Warn("re-capture ", busid, " (registry ", pending, "): ", err)
		h.access.Lock()
		delete(h.exports, busid)
		h.access.Unlock()
		if exp.device != nil {
			exp.device.Close()
		}
		return true, nil
	}
	info := device.info
	replacement := &darwinExport{
		busid:      info.key.BusID,
		registryID: info.registryID,
		device:     device,
		entry:      info.entry,
		logger:     h.logger,
	}
	h.access.Lock()
	h.exports[busid] = replacement
	h.access.Unlock()
	if exp.device != nil {
		exp.device.Close()
	}
	h.logger.Info("re-exported ", busid, " through IOUSBHost re-capture (registry ", pending, ")")
	return true, nil
}

func (h *darwinExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	return snapshotDarwinExports(h.exports)
}

func snapshotDarwinExports(exports map[string]*darwinExport) map[string]Export {
	out := make(map[string]Export, len(exports))
	for busid, exp := range exports {
		out[busid] = exp
	}
	return out
}

func cloneDarwinExport(exp *darwinExport) *darwinExport {
	if exp == nil {
		return nil
	}
	clone := *exp
	clone.entry.Interfaces = slices.Clone(exp.entry.Interfaces)
	return &clone
}

type darwinExport struct {
	busid             string
	registryID        uint64
	pendingRegistryID uint64
	device            *darwinUSBHostDevice
	entry             DeviceEntry
	logger            log.ContextLogger
	stale             bool
}

type darwinStaleMark struct {
	busid             string
	pendingRegistryID uint64
}

func (e *darwinExport) BusID() string {
	return e.busid
}

func (e *darwinExport) staleReason() string {
	if e.pendingRegistryID != 0 {
		return "device replaced"
	}
	return "capture released"
}

func (e *darwinExport) Snapshot(busy bool) ExportSnapshot {
	stableID := fmt.Sprintf("darwin-registry:%016x", e.registryID)
	if e.stale {
		return ExportSnapshot{
			Entry:        e.entry,
			Backend:      backendIDDarwinIOKit,
			StableID:     stableID,
			State:        deviceStateUnavailable,
			StatusReason: e.staleReason(),
		}
	}
	state := deviceStateAvailable
	if busy {
		state = deviceStateBusy
	}
	return ExportSnapshot{
		Entry:    e.entry,
		Backend:  backendIDDarwinIOKit,
		StableID: stableID,
		State:    state,
	}
}

func (e *darwinExport) DeviceInfo() (DeviceInfoTruncated, error) {
	return e.entry.Info, nil
}

func (e *darwinExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	return newUserspaceURBSession(ctx, e.logger, conn, newDarwinIOUSBHostEngine(e.device)), nil
}

type darwinImportHost struct {
	logger log.ContextLogger
}

func (h *darwinImportHost) Start() error {
	return nil
}

func (h *darwinImportHost) Close() error {
	return nil
}

func (h *darwinImportHost) Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error) {
	controller := newDarwinVirtualController(ctx, h.logger, conn, info)
	err := controller.Start()
	if err != nil {
		_ = controller.Close()
		return nil, err
	}
	return controller, nil
}
