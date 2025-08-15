package local

import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/service/resolved"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

type DBusResolvedResolver struct {
	logger           logger.ContextLogger
	interfaceMonitor tun.DefaultInterfaceMonitor
	systemBus        *dbus.Conn
	resoledObject    common.TypedValue[dbus.BusObject]
	closeOnce        sync.Once
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
		logger:           logger,
		interfaceMonitor: interfaceMonitor,
		systemBus:        systemBus,
	}, nil
}

func (t *DBusResolvedResolver) Start() error {
	t.updateStatus()
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
		if t.systemBus != nil {
			_ = t.systemBus.Close()
		}
	})
	return nil
}

func (t *DBusResolvedResolver) Object() any {
	return t.resoledObject.Load()
}

func (t *DBusResolvedResolver) Exchange(object any, ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	defaultInterface := t.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return nil, E.New("missing default interface")
	}
	question := message.Question[0]
	call := object.(*dbus.Object).CallWithContext(
		ctx,
		"org.freedesktop.resolve1.Manager.ResolveRecord",
		0,
		int32(defaultInterface.Index),
		question.Name,
		question.Qclass,
		question.Qtype,
		uint64(0),
	)
	if call.Err != nil {
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
	dbusObject := t.systemBus.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")
	err := dbusObject.Call("org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		if t.resoledObject.Swap(nil) != nil {
			t.logger.Debug("systemd-resolved service is gone")
		}
		return
	}
	t.resoledObject.Store(dbusObject)
	t.logger.Debug("using systemd-resolved service as resolver")
}
