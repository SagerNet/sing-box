package libbox

import (
	"encoding/binary"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

func readError(reader io.Reader) error {
	var hasError bool
	err := binary.Read(reader, binary.BigEndian, &hasError)
	if err != nil {
		return err
	}
	if hasError {
		errorMessage, err := varbin.ReadValue[string](reader, binary.BigEndian)
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
		err = varbin.Write(writer, binary.BigEndian, wErr.Error())
		if err != nil {
			return err
		}
	}
	return nil
}
