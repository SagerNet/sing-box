package libbox

import (
	"encoding/binary"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

func readError(reader io.Reader) error {
	var hasError bool
	err := binary.Read(reader, binary.BigEndian, &hasError)
	if err != nil {
		return err
	}
	if hasError {
		errorMessage, err := rw.ReadVString(reader)
		if err != nil {
			return err
		}
		return E.New(errorMessage)
	}
	return nil
}

func writeError(writer io.Writer, wErr error) error {
	err := binary.Write(writer, binary.BigEndian, wErr != nil)
	if err != nil {
		return err
	}
	if wErr != nil {
		err = rw.WriteVString(writer, wErr.Error())
		if err != nil {
			return err
		}
	}
	return nil
}
