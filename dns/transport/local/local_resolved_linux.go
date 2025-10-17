package local

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/service/resolved"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

func isSystemdResolvedManaged() bool {
	resolvContent, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return false
	}
	defer resolvContent.Close()
	scanner := bufio.NewScanner(resolvContent)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '#' {
			return false
		}
		if strings.Contains(line, "systemd-resolved") {
			return true
		}
	}
	return false
}

type DBusResolvedResolver struct {
	ctx               context.Context
	logger            logger.ContextLogger
	interfaceMonitor  tun.DefaultInterfaceMonitor
	interfaceCallback *list.Element[tun.DefaultInterfaceUpdateCallback]
	systemBus         *dbus.Conn
	resoledObject     atomic.Pointer[ResolvedObject]
	closeOnce         sync.Once
}

type ResolvedObject struct {
	dbus.BusObject
	InterfaceIndex int32
}

func NewResolvedResolver(ctx context.Context, logger logger.ContextLogger) (ResolvedResolver, error) {
	interfaceMonitor := service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, os.ErrInvalid
	}
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return &DBusResolvedResolver{
		ctx:              ctx,
		logger:           logger,
		interfaceMonitor: interfaceMonitor,
		systemBus:        systemBus,
	}, nil
}

func (t *DBusResolvedResolver) Start() error {
	t.updateStatus()
	t.interfaceCallback = t.interfaceMonitor.RegisterCallback(t.updateDefaultInterface)
	err := t.systemBus.BusObject().AddMatchSignal(
		"org.freedesktop.DBus",
		"NameOwnerChanged",
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchArg(0, "org.freedesktop.resolve1.Manager"),
	).Err
	if err != nil {
		return E.Cause(err, "configure resolved restart listener")
	}
	go t.loopUpdateStatus()
	return nil
}

func (t *DBusResolvedResolver) Close() error {
	t.closeOnce.Do(func() {
		if t.interfaceCallback != nil {
			t.interfaceMonitor.UnregisterCallback(t.interfaceCallback)
		}
		if t.systemBus != nil {
			_ = t.systemBus.Close()
		}
	})
	return nil
}

func (t *DBusResolvedResolver) Object() any {
	return common.PtrOrNil(t.resoledObject.Load())
}

func (t *DBusResolvedResolver) Exchange(object any, ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	resolvedObject := object.(*ResolvedObject)
	call := resolvedObject.CallWithContext(
		ctx,
		"org.freedesktop.resolve1.Manager.ResolveRecord",
		0,
		resolvedObject.InterfaceIndex,
		question.Name,
		question.Qclass,
		question.Qtype,
		uint64(0),
	)
	if call.Err != nil {
		var dbusError dbus.Error
		if errors.As(call.Err, &dbusError) && dbusError.Name == "org.freedesktop.resolve1.NoNameServers" {
			t.updateStatus()
		}
		return nil, E.Cause(call.Err, " resolve record via resolved")
	}
	var (
		records  []resolved.ResourceRecord
		outflags uint64
	)
	err := call.Store(&records, &outflags)
	if err != nil {
		return nil, err
	}
	response := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:                 message.Id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              mDNS.RcodeSuccess,
		},
		Question: []mDNS.Question{question},
	}
	for _, record := range records {
		var rr mDNS.RR
		rr, _, err = mDNS.UnpackRR(record.Data, 0)
		if err != nil {
			return nil, E.Cause(err, "unpack resource record")
		}
		response.Answer = append(response.Answer, rr)
	}
	return response, nil
}

func (t *DBusResolvedResolver) loopUpdateStatus() {
	signalChan := make(chan *dbus.Signal, 1)
	t.systemBus.Signal(signalChan)
	for signal := range signalChan {
		var restarted bool
		if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" {
			if len(signal.Body) != 3 || signal.Body[2].(string) == "" {
				continue
			} else {
				restarted = true
			}
		}
		if restarted {
			t.updateStatus()
		}
	}
}

func (t *DBusResolvedResolver) updateStatus() {
	dbusObject, err := t.checkResolved(context.Background())
	oldValue := t.resoledObject.Swap(dbusObject)
	if err != nil {
		var dbusErr dbus.Error
		if !errors.As(err, &dbusErr) || dbusErr.Name != "org.freedesktop.DBus.Error.NameHasNoOwnerCould" {
			t.logger.Debug(E.Cause(err, "systemd-resolved service unavailable"))
		}
		if oldValue != nil {
			t.logger.Debug("systemd-resolved service is gone")
		}
		return
	} else if oldValue == nil {
		t.logger.Debug("using systemd-resolved service as resolver")
	}
}

func (t *DBusResolvedResolver) checkResolved(ctx context.Context) (*ResolvedObject, error) {
	dbusObject := t.systemBus.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")
	err := dbusObject.Call("org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		return nil, err
	}
	defaultInterface := t.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return nil, E.New("missing default interface")
	}
	call := dbusObject.(*dbus.Object).CallWithContext(
		ctx,
		"org.freedesktop.resolve1.Manager.GetLink",
		0,
		int32(defaultInterface.Index),
	)
	if call.Err != nil {
		return nil, call.Err
	}
	var linkPath dbus.ObjectPath
	err = call.Store(&linkPath)
	if err != nil {
		return nil, err
	}
	linkObject := t.systemBus.Object("org.freedesktop.resolve1", linkPath)
	if linkObject == nil {
		return nil, E.New("missing link object for default interface")
	}
	dnsProp, err := linkObject.GetProperty("org.freedesktop.resolve1.Link.DNS")
	if err != nil {
		return nil, err
	}
	var linkDNS []resolved.LinkDNS
	err = dnsProp.Store(&linkDNS)
	if err != nil {
		return nil, err
	}
	if len(linkDNS) == 0 {
		for _, inbound := range service.FromContext[adapter.InboundManager](t.ctx).Inbounds() {
			if inbound.Type() == C.TypeTun {
				return nil, E.New("No appropriate name servers or networks for name found")
			}
		}
		return nil, E.New("link has no DNS servers configured")
	}
	return &ResolvedObject{
		BusObject:      dbusObject,
		InterfaceIndex: int32(defaultInterface.Index),
	}, nil
}

func (t *DBusResolvedResolver) updateDefaultInterface(defaultInterface *control.Interface, flags int) {
	t.updateStatus()
}
