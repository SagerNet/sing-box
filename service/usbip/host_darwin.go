//go:build darwin && cgo

package usbip

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// darwinExportHost adapts the macOS IOKit/IOUSBHost pipeline to the
// ExportHost interface. Captured devices stay owned by the host until
// release, including ones marked stale because they were busy when
// reconcile wanted to drop them.
type darwinExportHost struct {
	logger  log.ContextLogger
	matches []option.USBIPDeviceMatch
	ops     darwinServerOps

	access  sync.Mutex
	exports map[string]*darwinExport
	watcher darwinUSBHostDeviceWatch
}

func newDarwinExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch, ops darwinServerOps) *darwinExportHost {
	return &darwinExportHost{
		logger:  logger,
		matches: matches,
		ops:     ops,
		exports: make(map[string]*darwinExport),
	}
}

func (h *darwinExportHost) Start(ctx context.Context) error {
	return nil
}

func (h *darwinExportHost) Close() error {
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

func (h *darwinExportHost) Events(ctx context.Context) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	signal := func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	watcher, err := h.ops.watchUSBHostDevices(signal)
	if err != nil {
		close(ch)
		return nil, err
	}
	h.access.Lock()
	h.watcher = watcher
	h.access.Unlock()
	go func() {
		<-ctx.Done()
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

func (h *darwinExportHost) Reconcile(ctx context.Context, isBusy func(busid string) bool) (map[string]Export, []string, bool, error) {
	devices, err := h.ops.copyUSBHostDevices()
	if err != nil {
		return h.snapshotSelf(), nil, false, E.Cause(err, "enumerate IOUSBHost devices")
	}
	desired := make(map[string]darwinUSBHostDeviceInfo)
	for _, match := range h.matches {
		for i := range devices {
			if !matches(match, devices[i].key) {
				continue
			}
			if devices[i].entry.Info.BDeviceClass == 0x09 {
				h.logger.Warn("skip hub device ", devices[i].key.BusID, " matched by ", describeMatch(match))
				continue
			}
			desired[devices[i].key.BusID] = devices[i]
		}
	}

	h.access.Lock()
	current := make(map[string]*darwinExport, len(h.exports))
	for busid, exp := range h.exports {
		current[busid] = exp
	}
	h.access.Unlock()

	var (
		toAdd    []*darwinExport
		toRemove []*darwinExport
		toStale  []string
		released []string
	)
	for busid, info := range desired {
		if exp, ok := current[busid]; ok && exp.registryID == info.registryID {
			continue
		}
		if exp, ok := current[busid]; ok {
			if isBusy(busid) {
				toStale = append(toStale, busid)
				continue
			}
			toRemove = append(toRemove, exp)
			released = append(released, busid)
		}
		device, err := h.ops.openUSBHostDevice(info.registryID, true)
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
		if isBusy(busid) {
			toStale = append(toStale, busid)
			continue
		}
		toRemove = append(toRemove, exp)
		h.logger.Info("released ", busid, " from IOUSBHost capture")
		released = append(released, busid)
	}

	changed := len(toAdd) > 0 || len(toRemove) > 0
	h.access.Lock()
	for _, busid := range toStale {
		exp, ok := h.exports[busid]
		if !ok || exp.stale {
			continue
		}
		exp.stale = true
		changed = true
	}
	for _, exp := range toRemove {
		delete(h.exports, exp.busid)
	}
	for _, exp := range toAdd {
		h.exports[exp.busid] = exp
	}
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		if exp.stale {
			continue
		}
		out[busid] = exp
	}
	h.access.Unlock()

	for _, exp := range toRemove {
		if exp.device != nil {
			exp.device.Close()
		}
	}
	return out, released, changed, nil
}

func (h *darwinExportHost) FinishImport(ctx context.Context, busid string) (bool, error) {
	h.access.Lock()
	exp, ok := h.exports[busid]
	if !ok || !exp.stale {
		h.access.Unlock()
		return false, nil
	}
	delete(h.exports, busid)
	h.access.Unlock()
	if exp.device != nil {
		exp.device.Close()
	}
	return true, nil
}

func (h *darwinExportHost) snapshotSelf() map[string]Export {
	h.access.Lock()
	defer h.access.Unlock()
	out := make(map[string]Export, len(h.exports))
	for busid, exp := range h.exports {
		if exp.stale {
			continue
		}
		out[busid] = exp
	}
	return out
}

type darwinExport struct {
	busid      string
	registryID uint64
	device     *darwinUSBHostDevice
	entry      DeviceEntry
	logger     log.ContextLogger
	stale      bool
}

func (e *darwinExport) BusID() string {
	return e.busid
}

func (e *darwinExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	stableID := darwinStableID(e.registryID)
	if e.stale {
		return ExportSnapshot{
			Backend:  backendIDDarwinIOKit,
			StableID: stableID,
			State:    deviceStateUnavailable,
		}
	}
	state := deviceStateAvailable
	reason := deviceStateAvailable
	if busy {
		state = deviceStateBusy
		reason = deviceStateBusy
	}
	return ExportSnapshot{
		Entry:        e.entry,
		Backend:      backendIDDarwinIOKit,
		StableID:     stableID,
		State:        state,
		StatusReason: reason,
	}
}

func (e *darwinExport) LeaseCheck(ctx context.Context) (bool, string) {
	if e.stale {
		return false, deviceStateUnavailable
	}
	return true, ""
}

func (e *darwinExport) DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error) {
	return e.entry.Info, nil
}

func (e *darwinExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	if e.device == nil {
		return nil, E.New("darwin export ", e.busid, " has no device handle")
	}
	return newDarwinServerDataSession(ctx, e.logger, conn, e.device), nil
}

// darwinImportHost adapts the macOS userspace IOUSBHostControllerInterface
// pipeline to the ImportHost interface.
type darwinImportHost struct {
	logger log.ContextLogger
}

func newDarwinImportHost(logger log.ContextLogger) *darwinImportHost {
	return &darwinImportHost{logger: logger}
}

func (h *darwinImportHost) Start(ctx context.Context) error {
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
	return &darwinClientSession{controller: controller}, nil
}

type darwinClientSession struct {
	controller *darwinVirtualController
}

func (s *darwinClientSession) Done() <-chan struct{} {
	return s.controller.Done()
}

func (s *darwinClientSession) Err() error {
	return s.controller.Err()
}

func (s *darwinClientSession) Close() error {
	return s.controller.Close()
}

func (s *darwinClientSession) Description() string {
	return "IOUSBHostControllerInterface"
}
