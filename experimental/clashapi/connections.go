package clashapi

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/trafficcontrol"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gofrs/uuid/v5"
)

func connectionRouter(ctx context.Context, network adapter.NetworkManager, trafficManager *trafficcontrol.Manager) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConnections(ctx, trafficManager))
	r.Delete("/", closeAllConnections(network, trafficManager))
	r.Delete("/{id}", closeConnection(trafficManager))
	return r
}

func connectionsSnapshot(trafficManager *trafficcontrol.Manager) render.M {
	uplinkTotal, downlinkTotal := trafficManager.Total()
	connections := common.Filter(trafficManager.Connections(), func(metadata *trafficcontrol.TrackerMetadata) bool {
		return metadata.OutboundType != C.TypeDNS
	})
	return render.M{
		"downloadTotal": downlinkTotal,
		"uploadTotal":   uplinkTotal,
		"connections": common.Map(connections, func(metadata *trafficcontrol.TrackerMetadata) connectionObject {
			return connectionObject(*metadata)
		}),
		"memory": inuseMemory(),
	}
}

type connectionObject trafficcontrol.TrackerMetadata

func (c connectionObject) MarshalJSON() ([]byte, error) {
	var inbound string
	if c.Metadata.Inbound != "" {
		inbound = c.Metadata.InboundType + "/" + c.Metadata.Inbound
	} else {
		inbound = c.Metadata.InboundType
	}
	var domain string
	if c.Metadata.Domain != "" {
		domain = c.Metadata.Domain
	} else {
		domain = c.Metadata.Destination.Fqdn
	}
	var processPath string
	if c.Metadata.ProcessInfo != nil {
		if c.Metadata.ProcessInfo.ProcessPath != "" {
			processPath = c.Metadata.ProcessInfo.ProcessPath
		} else if len(c.Metadata.ProcessInfo.AndroidPackageNames) > 0 {
			processPath = c.Metadata.ProcessInfo.AndroidPackageNames[0]
		}
		if processPath == "" {
			if c.Metadata.ProcessInfo.UserId != -1 {
				processPath = F.ToString(c.Metadata.ProcessInfo.UserId)
			}
		} else if c.Metadata.ProcessInfo.UserName != "" {
			processPath = F.ToString(processPath, " (", c.Metadata.ProcessInfo.UserName, ")")
		} else if c.Metadata.ProcessInfo.UserId != -1 {
			processPath = F.ToString(processPath, " (", c.Metadata.ProcessInfo.UserId, ")")
		}
	}
	var rule string
	if c.Rule != nil {
		rule = F.ToString(c.Rule, " => ", c.Rule.Action())
	} else {
		rule = "final"
	}
	return json.Marshal(map[string]any{
		"id": c.ID,
		"metadata": map[string]any{
			"network":         c.Metadata.Network,
			"type":            inbound,
			"sourceIP":        c.Metadata.Source.Addr,
			"destinationIP":   c.Metadata.Destination.Addr,
			"sourcePort":      F.ToString(c.Metadata.Source.Port),
			"destinationPort": F.ToString(c.Metadata.Destination.Port),
			"host":            domain,
			"dnsMode":         "normal",
			"processPath":     processPath,
		},
		"upload":      c.Upload.Load(),
		"download":    c.Download.Load(),
		"start":       c.CreatedAt,
		"chains":      c.Chain,
		"rule":        rule,
		"rulePayload": "",
	})
}

func getConnections(ctx context.Context, trafficManager *trafficcontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			render.JSON(w, r, connectionsSnapshot(trafficManager))
			return
		}

		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()

		intervalStr := r.URL.Query().Get("interval")
		interval := 1000
		if intervalStr != "" {
			t, err := strconv.Atoi(intervalStr)
			if err != nil {
				render.Status(r, http.StatusBadRequest)
				render.JSON(w, r, ErrBadRequest)
				return
			}

			interval = t
		}

		buf := &bytes.Buffer{}
		sendSnapshot := func() error {
			buf.Reset()
			encodeErr := json.NewEncoder(buf).Encode(connectionsSnapshot(trafficManager))
			if encodeErr != nil {
				return encodeErr
			}
			return wsutil.WriteServerText(conn, buf.Bytes())
		}

		if err = sendSnapshot(); err != nil {
			return
		}

		tick := time.NewTicker(time.Millisecond * time.Duration(interval))
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			if err = sendSnapshot(); err != nil {
				break
			}
		}
	}
}

func closeConnection(trafficManager *trafficcontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := uuid.FromStringOrNil(chi.URLParam(r, "id"))
		targetConnection := trafficManager.Connection(id)
		if targetConnection != nil {
			targetConnection.Close()
		}
		render.NoContent(w, r)
	}
}

func closeAllConnections(network adapter.NetworkManager, trafficManager *trafficcontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		trafficManager.CloseAllConnections()
		network.ResetNetwork()
		render.NoContent(w, r)
	}
}
