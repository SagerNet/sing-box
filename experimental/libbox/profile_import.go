package libbox

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

func EncodeChunkedMessage(data []byte) []byte {
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.BigEndian, uint16(len(data)))
	buffer.Write(data)
	return buffer.Bytes()
}

func DecodeLengthChunk(data []byte) int32 {
	return int32(binary.BigEndian.Uint16(data))
}

const (
	MessageTypeError = iota
	MessageTypeProfileList
	MessageTypeProfileContentRequest
	MessageTypeProfileContent
)

type ErrorMessage struct {
	Message string
}

func (e *ErrorMessage) Encode() []byte {
	var buffer bytes.Buffer
	buffer.WriteByte(MessageTypeError)
	rw.WriteVString(&buffer, e.Message)
	return buffer.Bytes()
}

func DecodeErrorMessage(data []byte) (*ErrorMessage, error) {
	reader := bytes.NewReader(data)
	messageType, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if messageType != MessageTypeError {
		return nil, E.New("invalid message")
	}
	var message ErrorMessage
	message.Message, err = rw.ReadVString(reader)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

const (
	ProfileTypeLocal int32 = iota
	ProfileTypeiCloud
	ProfileTypeRemote
)

type ProfilePreview struct {
	ProfileID int64
	Name      string
	Type      int32
}

type ProfilePreviewIterator interface {
	Next() *ProfilePreview
	HasNext() bool
}

type ProfileEncoder struct {
	profiles []ProfilePreview
}

func (e *ProfileEncoder) Append(profile *ProfilePreview) {
	e.profiles = append(e.profiles, *profile)
}

func (e *ProfileEncoder) Encode() []byte {
	var buffer bytes.Buffer
	buffer.WriteByte(MessageTypeProfileList)
	binary.Write(&buffer, binary.BigEndian, uint16(len(e.profiles)))
	for _, preview := range e.profiles {
		binary.Write(&buffer, binary.BigEndian, preview.ProfileID)
		rw.WriteVString(&buffer, preview.Name)
		binary.Write(&buffer, binary.BigEndian, preview.Type)
	}
	return buffer.Bytes()
}

type ProfileDecoder struct {
	profiles []*ProfilePreview
}

func (d *ProfileDecoder) Decode(data []byte) error {
	reader := bytes.NewReader(data)
	messageType, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if messageType != MessageTypeProfileList {
		return E.New("invalid message")
	}
	var profileCount uint16
	err = binary.Read(reader, binary.BigEndian, &profileCount)
	if err != nil {
		return err
	}
	for i := 0; i < int(profileCount); i++ {
		var profile ProfilePreview
		err = binary.Read(reader, binary.BigEndian, &profile.ProfileID)
		if err != nil {
			return err
		}
		profile.Name, err = rw.ReadVString(reader)
		if err != nil {
			return err
		}
		err = binary.Read(reader, binary.BigEndian, &profile.Type)
		if err != nil {
			return err
		}
		d.profiles = append(d.profiles, &profile)
	}
	return nil
}

func (d *ProfileDecoder) Iterator() ProfilePreviewIterator {
	return newIterator(d.profiles)
}

type ProfileContentRequest struct {
	ProfileID int64
}

func (r *ProfileContentRequest) Encode() []byte {
	var buffer bytes.Buffer
	buffer.WriteByte(MessageTypeProfileContentRequest)
	binary.Write(&buffer, binary.BigEndian, r.ProfileID)
	return buffer.Bytes()
}

func DecodeProfileContentRequest(data []byte) (*ProfileContentRequest, error) {
	reader := bytes.NewReader(data)
	messageType, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if messageType != MessageTypeProfileContentRequest {
		return nil, E.New("invalid message")
	}
	var request ProfileContentRequest
	err = binary.Read(reader, binary.BigEndian, &request.ProfileID)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

type ProfileContent struct {
	Name        string
	Type        int32
	Config      string
	RemotePath  string
	AutoUpdate  bool
	LastUpdated int64
}

func (c *ProfileContent) Encode() []byte {
	buffer := new(bytes.Buffer)
	buffer.WriteByte(MessageTypeProfileContent)
	buffer.WriteByte(0)
	writer := gzip.NewWriter(buffer)
	rw.WriteVString(writer, c.Name)
	binary.Write(writer, binary.BigEndian, c.Type)
	rw.WriteVString(writer, c.Config)
	if c.Type != ProfileTypeLocal {
		rw.WriteVString(writer, c.RemotePath)
		binary.Write(writer, binary.BigEndian, c.AutoUpdate)
		binary.Write(writer, binary.BigEndian, c.LastUpdated)
	}
	writer.Flush()
	writer.Close()
	return buffer.Bytes()
}

func DecodeProfileContent(data []byte) (*ProfileContent, error) {
	var reader io.Reader = bytes.NewReader(data)
	messageType, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if messageType != MessageTypeProfileContent {
		return nil, E.New("invalid message")
	}
	version, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, E.Cause(err, "unsupported profile")
		}
	} else {
		return nil, E.Cause(err, "unsupported profile")
	}
	var content ProfileContent
	content.Name, err = rw.ReadVString(reader)
	if err != nil {
		return nil, err
	}
	err = binary.Read(reader, binary.BigEndian, &content.Type)
	if err != nil {
		return nil, err
	}
	content.Config, err = rw.ReadVString(reader)
	if err != nil {
		return nil, err
	}
	if content.Type != ProfileTypeLocal {
		content.RemotePath, err = rw.ReadVString(reader)
		if err != nil {
			return nil, err
		}
		err = binary.Read(reader, binary.BigEndian, &content.AutoUpdate)
		if err != nil {
			return nil, err
		}
		err = binary.Read(reader, binary.BigEndian, &content.LastUpdated)
		if err != nil {
			return nil, err
		}
	}
	return &content, nil
}
