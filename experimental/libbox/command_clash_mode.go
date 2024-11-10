package libbox

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashapi"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

func (c *CommandClient) SetClashMode(newMode string) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandSetClashMode))
	if err != nil {
		return err
	}
	err = varbin.Write(conn, binary.BigEndian, newMode)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSetClashMode(conn net.Conn) error {
	newMode, err := varbin.ReadValue[string](conn, binary.BigEndian)
	if err != nil {
		return err
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	service.clashServer.(*clashapi.Server).SetMode(newMode)
	return writeError(conn, nil)
}

func (c *CommandClient) handleModeConn(conn net.Conn) {
	defer conn.Close()

	for {
		newMode, err := varbin.ReadValue[string](conn, binary.BigEndian)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.UpdateClashMode(newMode)
	}
}

func (s *CommandServer) handleModeConn(conn net.Conn) error {
	ctx := connKeepAlive(conn)
	for s.service == nil {
		select {
		case <-time.After(time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	err := writeClashModeList(conn, s.service.clashServer)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.modeUpdate:
			err = varbin.Write(conn, binary.BigEndian, s.service.clashServer.Mode())
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func readClashModeList(reader io.Reader) (modeList []string, currentMode string, err error) {
	var modeListLength uint16
	err = binary.Read(reader, binary.BigEndian, &modeListLength)
	if err != nil {
		return
	}
	if modeListLength == 0 {
		return
	}
	modeList = make([]string, modeListLength)
	for i := 0; i < int(modeListLength); i++ {
		modeList[i], err = varbin.ReadValue[string](reader, binary.BigEndian)
		if err != nil {
			return
		}
	}
	currentMode, err = varbin.ReadValue[string](reader, binary.BigEndian)
	return
}

func writeClashModeList(writer io.Writer, clashServer adapter.ClashServer) error {
	modeList := clashServer.ModeList()
	err := binary.Write(writer, binary.BigEndian, uint16(len(modeList)))
	if err != nil {
		return err
	}
	if len(modeList) > 0 {
		for _, mode := range modeList {
			err = varbin.Write(writer, binary.BigEndian, mode)
			if err != nil {
				return err
			}
		}
		err = varbin.Write(writer, binary.BigEndian, clashServer.Mode())
		if err != nil {
			return err
		}
	}
	return nil
}
