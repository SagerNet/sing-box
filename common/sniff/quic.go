package sniff

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"encoding/binary"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff/internal/qtls"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/crypto/hkdf"
)

func QUICClientHello(ctx context.Context, packet []byte) (*adapter.InboundContext, error) {
	reader := bytes.NewReader(packet)

	typeByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if typeByte&0x80 == 0 || typeByte&0x40 == 0 {
		return nil, E.New("bad type byte")
	}
	var versionNumber uint32
	err = binary.Read(reader, binary.BigEndian, &versionNumber)
	if err != nil {
		return nil, err
	}
	if versionNumber != qtls.VersionDraft29 && versionNumber != qtls.Version1 && versionNumber != qtls.Version2 {
		return nil, E.New("bad version")
	}
	packetType := (typeByte & 0x30) >> 4
	if packetType == 0 && versionNumber == qtls.Version2 || packetType == 2 && versionNumber != qtls.Version2 || packetType > 2 {
		return nil, E.New("bad packet type")
	}

	destConnIDLen, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if destConnIDLen == 0 || destConnIDLen > 20 {
		return nil, E.New("bad destination connection id length")
	}

	destConnID := make([]byte, destConnIDLen)
	_, err = io.ReadFull(reader, destConnID)
	if err != nil {
		return nil, err
	}

	srcConnIDLen, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	_, err = io.CopyN(io.Discard, reader, int64(srcConnIDLen))
	if err != nil {
		return nil, err
	}

	tokenLen, err := qtls.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}

	_, err = io.CopyN(io.Discard, reader, int64(tokenLen))
	if err != nil {
		return nil, err
	}

	packetLen, err := qtls.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}

	hdrLen := int(reader.Size()) - reader.Len()
	if hdrLen+int(packetLen) > len(packet) {
		return nil, os.ErrInvalid
	}

	_, err = io.CopyN(io.Discard, reader, 4)
	if err != nil {
		return nil, err
	}

	pnBytes := make([]byte, aes.BlockSize)
	_, err = io.ReadFull(reader, pnBytes)
	if err != nil {
		return nil, err
	}

	var salt []byte
	switch versionNumber {
	case qtls.Version1:
		salt = qtls.SaltV1
	case qtls.Version2:
		salt = qtls.SaltV2
	default:
		salt = qtls.SaltOld
	}
	var hkdfHeaderProtectionLabel string
	switch versionNumber {
	case qtls.Version2:
		hkdfHeaderProtectionLabel = qtls.HKDFLabelHeaderProtectionV2
	default:
		hkdfHeaderProtectionLabel = qtls.HKDFLabelHeaderProtectionV1
	}
	initialSecret := hkdf.Extract(crypto.SHA256.New, destConnID, salt)
	secret := qtls.HKDFExpandLabel(crypto.SHA256, initialSecret, []byte{}, "client in", crypto.SHA256.Size())
	hpKey := qtls.HKDFExpandLabel(crypto.SHA256, secret, []byte{}, hkdfHeaderProtectionLabel, 16)
	block, err := aes.NewCipher(hpKey)
	if err != nil {
		return nil, err
	}
	mask := make([]byte, aes.BlockSize)
	block.Encrypt(mask, pnBytes)
	newPacket := make([]byte, len(packet))
	copy(newPacket, packet)
	newPacket[0] ^= mask[0] & 0xf
	for i := range newPacket[hdrLen : hdrLen+4] {
		newPacket[hdrLen+i] ^= mask[i+1]
	}
	packetNumberLength := newPacket[0]&0x3 + 1
	if hdrLen+int(packetNumberLength) > int(packetLen)+hdrLen {
		return nil, os.ErrInvalid
	}
	var packetNumber uint32
	switch packetNumberLength {
	case 1:
		packetNumber = uint32(newPacket[hdrLen])
	case 2:
		packetNumber = uint32(binary.BigEndian.Uint16(newPacket[hdrLen:]))
	case 3:
		packetNumber = uint32(newPacket[hdrLen+2]) | uint32(newPacket[hdrLen+1])<<8 | uint32(newPacket[hdrLen])<<16
	case 4:
		packetNumber = binary.BigEndian.Uint32(newPacket[hdrLen:])
	default:
		return nil, E.New("bad packet number length")
	}
	if packetNumber != 0 {
		return nil, E.New("bad packet number: ", packetNumber)
	}
	extHdrLen := hdrLen + int(packetNumberLength)
	copy(newPacket[extHdrLen:hdrLen+4], packet[extHdrLen:])
	data := newPacket[extHdrLen : int(packetLen)+hdrLen]

	var keyLabel string
	var ivLabel string
	switch versionNumber {
	case qtls.Version2:
		keyLabel = qtls.HKDFLabelKeyV2
		ivLabel = qtls.HKDFLabelIVV2
	default:
		keyLabel = qtls.HKDFLabelKeyV1
		ivLabel = qtls.HKDFLabelIVV1
	}

	key := qtls.HKDFExpandLabel(crypto.SHA256, secret, []byte{}, keyLabel, 16)
	iv := qtls.HKDFExpandLabel(crypto.SHA256, secret, []byte{}, ivLabel, 12)
	cipher := qtls.AEADAESGCMTLS13(key, iv)
	nonce := make([]byte, int32(cipher.NonceSize()))
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:], uint64(packetNumber))
	decrypted, err := cipher.Open(newPacket[extHdrLen:extHdrLen], nonce, data, newPacket[:extHdrLen])
	if err != nil {
		return nil, err
	}
	decryptedReader := bytes.NewReader(decrypted)
	frameType, err := decryptedReader.ReadByte()
	if err != nil {
		return nil, err
	}
	for frameType == 0x0 {
		// skip padding
		frameType, err = decryptedReader.ReadByte()
		if err != nil {
			return nil, err
		}
	}
	if frameType != 0x6 {
		// not crypto frame
		return &adapter.InboundContext{Protocol: C.ProtocolQUIC}, nil
	}
	_, err = qtls.ReadUvarint(decryptedReader)
	if err != nil {
		return nil, err
	}
	_, err = qtls.ReadUvarint(decryptedReader)
	if err != nil {
		return nil, err
	}
	tlsHdr := make([]byte, 5)
	tlsHdr[0] = 0x16
	binary.BigEndian.PutUint16(tlsHdr[1:], uint16(0x0303))
	binary.BigEndian.PutUint16(tlsHdr[3:], uint16(decryptedReader.Len()))
	metadata, err := TLSClientHello(ctx, io.MultiReader(bytes.NewReader(tlsHdr), decryptedReader))
	if err != nil {
		return nil, err
	}
	metadata.Protocol = C.ProtocolQUIC
	return metadata, nil
}
