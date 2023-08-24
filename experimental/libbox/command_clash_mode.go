package libbox

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashapi"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
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
	err = rw.WriteVString(conn, newMode)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSetClashMode(conn net.Conn) error {
	defer conn.Close()
	newMode, err := rw.ReadVString(conn)
	if err != nil {
		return err
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	clashServer := service.instance.Router().ClashServer()
	if clashServer == nil {
		return writeError(conn, E.New("Clash API disabled"))
	}
	clashServer.(*clashapi.Server).SetMode(newMode)
	return writeError(conn, nil)
}

func (c *CommandClient) handleModeConn(conn net.Conn) {
	defer conn.Close()

	for {
		newMode, err := rw.ReadVString(conn)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.UpdateClashMode(newMode)
	}
}

func (s *CommandServer) handleModeConn(conn net.Conn) error {
	defer conn.Close()
	ctx := connKeepAlive(conn)
	for s.service == nil {
		select {
		case <-time.After(time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	clashServer := s.service.instance.Router().ClashServer()
	if clashServer == nil {
		defer conn.Close()
		return binary.Write(conn, binary.BigEndian, uint16(0))
	}
	err := writeClashModeList(conn, clashServer)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.modeUpdate:
			err = rw.WriteVString(conn, clashServer.Mode())
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
		modeList[i], err = rw.ReadVString(reader)
		if err != nil {
			return
		}
	}
	currentMode, err = rw.ReadVString(reader)
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
			err = rw.WriteVString(writer, mode)
			if err != nil {
				return err
			}
		}
		err = rw.WriteVString(writer, clashServer.Mode())
		if err != nil {
			return err
		}
	}
	return nil
}
