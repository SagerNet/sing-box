package main

import (
	"slices"
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPIConnectionListFlagColumns []string

type connectionRate struct {
	uplink   int64
	downlink int64
}

type connectionColumn struct {
	header string
	value  func(connection *daemon.Connection, rates map[string]connectionRate) string
}

var connectionColumnNames = []string{
	"id", "network", "source", "destination", "inbound", "outbound",
	"chain", "rule", "protocol", "user", "process", "created", "rate", "total",
}

var connectionColumns = map[string]connectionColumn{
	"id": {"ID", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetId()
	}},
	"network": {"NETWORK", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetNetwork()
	}},
	"source": {"SOURCE", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetSource()
	}},
	"destination": {"DESTINATION", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connectionDestination(connection)
	}},
	"inbound": {"INBOUND", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		if connection.GetInbound() == "" {
			return connection.GetInboundType()
		}
		return connection.GetInboundType() + "/" + connection.GetInbound()
	}},
	"outbound": {"OUTBOUND", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetOutbound()
	}},
	"chain": {"CHAIN", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		chain := slices.Clone(connection.GetChainList())
		slices.Reverse(chain)
		return strings.Join(chain, "/")
	}},
	"rule": {"RULE", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetRule()
	}},
	"protocol": {"PROTOCOL", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetProtocol()
	}},
	"user": {"USER", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return connection.GetUser()
	}},
	"process": {"PROCESS", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		processInfo := connection.GetProcessInfo()
		if processInfo.GetProcessPath() != "" {
			return processInfo.GetProcessPath()
		}
		if len(processInfo.GetPackageNames()) > 0 {
			return processInfo.GetPackageNames()[0]
		}
		return ""
	}},
	"created": {"CREATED", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return formatConnectionTime(connection.GetCreatedAt())
	}},
	"rate": {"RATE", func(connection *daemon.Connection, rates map[string]connectionRate) string {
		rate, found := rates[connection.GetId()]
		if !found || (rate.uplink == 0 && rate.downlink == 0) {
			return ""
		}
		return "↑" + byteformats.FormatBytes(uint64(rate.uplink)) + "/s ↓" + byteformats.FormatBytes(uint64(rate.downlink)) + "/s"
	}},
	"total": {"TOTAL", func(connection *daemon.Connection, _ map[string]connectionRate) string {
		return "↑" + byteformats.FormatBytes(uint64(connection.GetUplinkTotal())) + " ↓" + byteformats.FormatBytes(uint64(connection.GetDownlinkTotal()))
	}},
}

var commandAPIConnectionList = &cobra.Command{
	Use:   "list",
	Short: "List open connections",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIConnectionList()
	},
}

func init() {
	commandAPIConnectionList.Flags().StringSliceVar(
		&commandAPIConnectionListFlagColumns,
		"columns",
		[]string{"id", "network", "destination", "inbound", "outbound", "total"},
		"Columns to display (available: "+strings.Join(connectionColumnNames, ", ")+")",
	)
	commandAPIConnection.AddCommand(commandAPIConnectionList)
}

func runAPIConnectionList() error {
	columns := make([]connectionColumn, 0, len(commandAPIConnectionListFlagColumns))
	sampleRates := false
	for _, name := range commandAPIConnectionListFlagColumns {
		column, found := connectionColumns[name]
		if !found {
			return E.New("unknown column: ", name, ", available: ", strings.Join(connectionColumnNames, ", "))
		}
		if name == "rate" {
			sampleRates = true
		}
		columns = append(columns, column)
	}
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	var (
		connections []*daemon.Connection
		rates       map[string]connectionRate
	)
	if sampleRates {
		connections, rates, err = fetchConnectionsAndRates(client)
	} else {
		connections, err = fetchConnections(client)
	}
	if err != nil {
		return err
	}
	table := tableWriter{
		header: common.Map(columns, func(it connectionColumn) string {
			return it.header
		}),
		emptyMessage: "no connections",
	}
	for _, connection := range connections {
		if connection.GetClosedAt() != 0 {
			continue
		}
		table.addRow(common.Map(columns, func(it connectionColumn) string {
			return it.value(connection, rates)
		})...)
	}
	table.flush()
	return nil
}

func connectionDestination(connection *daemon.Connection) string {
	destination := connection.GetDestination()
	domain := connection.GetDomain()
	if domain == "" {
		return destination
	}
	portIndex := strings.LastIndex(destination, ":")
	if portIndex == -1 {
		return domain
	}
	return domain + destination[portIndex:]
}

func fetchConnectionsAndRates(client daemon.StartedServiceClient) ([]*daemon.Connection, map[string]connectionRate, error) {
	stream, err := client.SubscribeConnections(globalCtx, &daemon.SubscribeConnectionsRequest{Interval: int64(time.Second)})
	if err != nil {
		return nil, nil, err
	}
	initialEvents, err := stream.Recv()
	if err != nil {
		return nil, nil, err
	}
	connections := common.FilterNotNil(common.Map(initialEvents.GetEvents(), func(it *daemon.ConnectionEvent) *daemon.Connection {
		return it.GetConnection()
	}))
	common.SortBy(connections, func(it *daemon.Connection) int64 {
		return it.GetCreatedAt()
	})
	updateEvents, err := stream.Recv()
	if err != nil {
		return nil, nil, err
	}
	rates := make(map[string]connectionRate, len(updateEvents.GetEvents()))
	for _, event := range updateEvents.GetEvents() {
		rates[event.GetId()] = connectionRate{
			uplink:   event.GetUplinkDelta(),
			downlink: event.GetDownlinkDelta(),
		}
	}
	return connections, rates, nil
}
