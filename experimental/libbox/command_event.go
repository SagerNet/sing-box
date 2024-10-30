package libbox

import (
	"encoding/binary"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

type myEvent interface {
	writeTo(writer varbin.Writer)
}

func readEvent(reader varbin.Reader) (myEvent, error) {
	eventType, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch eventType {
	case eventTypeEmpty:
		return nil, nil
	case eventTypeOpenURL:
		url, err := varbin.ReadValue[string](reader, binary.BigEndian)
		if err != nil {
			return nil, err
		}
		return &eventOpenURL{URL: url}, nil
	default:
		return nil, E.New("unknown event type: ", eventType)
	}
}

type eventOpenURL struct {
	URL string
}

func (e *eventOpenURL) writeTo(writer varbin.Writer) {
	writer.WriteByte(eventTypeOpenURL)
	varbin.Write(writer, binary.BigEndian, e.URL)
}
