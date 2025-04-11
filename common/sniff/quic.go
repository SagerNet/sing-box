package sniff

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/tls"
	"encoding/binary"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/ja3"
	"github.com/sagernet/sing-box/common/sniff/internal/qtls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/crypto/hkdf"
)

func QUICClientHello(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	reader := bytes.NewReader(packet)
	typeByte, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if typeByte&0x40 == 0 {
		return E.New("bad type byte")
	}
	var versionNumber uint32
	err = binary.Read(reader, binary.BigEndian, &versionNumber)
	if err != nil {
		return err
	}
	if versionNumber != qtls.VersionDraft29 && versionNumber != qtls.Version1 && versionNumber != qtls.Version2 {
		return E.New("bad version")
	}
	packetType := (typeByte & 0x30) >> 4
	if packetType == 0 && versionNumber == qtls.Version2 || packetType == 2 && versionNumber != qtls.Version2 || packetType > 2 {
		return E.New("bad packet type")
	}

	destConnIDLen, err := reader.ReadByte()
	if err != nil {
		return err
	}

	if destConnIDLen == 0 || destConnIDLen > 20 {
		return E.New("bad destination connection id length")
	}

	destConnID := make([]byte, destConnIDLen)
	_, err = io.ReadFull(reader, destConnID)
	if err != nil {
		return err
	}

	srcConnIDLen, err := reader.ReadByte()
	if err != nil {
		return err
	}

	_, err = io.CopyN(io.Discard, reader, int64(srcConnIDLen))
	if err != nil {
		return err
	}

	tokenLen, err := qtls.ReadUvarint(reader)
	if err != nil {
		return err
	}

	_, err = io.CopyN(io.Discard, reader, int64(tokenLen))
	if err != nil {
		return err
	}

	packetLen, err := qtls.ReadUvarint(reader)
	if err != nil {
		return err
	}

	hdrLen := int(reader.Size()) - reader.Len()
	if hdrLen+int(packetLen) > len(packet) {
		return os.ErrInvalid
	}

	_, err = io.CopyN(io.Discard, reader, 4)
	if err != nil {
		return err
	}

	pnBytes := make([]byte, aes.BlockSize)
	_, err = io.ReadFull(reader, pnBytes)
	if err != nil {
		return err
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
		return err
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
		return os.ErrInvalid
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
		return E.New("bad packet number length")
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
		return err
	}
	var frameType byte
	var fragments []qCryptoFragment
	decryptedReader := bytes.NewReader(decrypted)
	const (
		frameTypePadding         = 0x00
		frameTypePing            = 0x01
		frameTypeAck             = 0x02
		frameTypeAck2            = 0x03
		frameTypeCrypto          = 0x06
		frameTypeConnectionClose = 0x1c
	)
	var frameTypeList []uint8
	for {
		frameType, err = decryptedReader.ReadByte()
		if err == io.EOF {
			break
		}
		frameTypeList = append(frameTypeList, frameType)
		switch frameType {
		case frameTypePadding:
			continue
		case frameTypePing:
			continue
		case frameTypeAck, frameTypeAck2:
			_, err = qtls.ReadUvarint(decryptedReader) // Largest Acknowledged
			if err != nil {
				return err
			}
			_, err = qtls.ReadUvarint(decryptedReader) // ACK Delay
			if err != nil {
				return err
			}
			ackRangeCount, err := qtls.ReadUvarint(decryptedReader) // ACK Range Count
			if err != nil {
				return err
			}
			_, err = qtls.ReadUvarint(decryptedReader) // First ACK Range
			if err != nil {
				return err
			}
			for i := 0; i < int(ackRangeCount); i++ {
				_, err = qtls.ReadUvarint(decryptedReader) // Gap
				if err != nil {
					return err
				}
				_, err = qtls.ReadUvarint(decryptedReader) // ACK Range Length
				if err != nil {
					return err
				}
			}
			if frameType == 0x03 {
				_, err = qtls.ReadUvarint(decryptedReader) // ECT0 Count
				if err != nil {
					return err
				}
				_, err = qtls.ReadUvarint(decryptedReader) // ECT1 Count
				if err != nil {
					return err
				}
				_, err = qtls.ReadUvarint(decryptedReader) // ECN-CE Count
				if err != nil {
					return err
				}
			}
		case frameTypeCrypto:
			var offset uint64
			offset, err = qtls.ReadUvarint(decryptedReader)
			if err != nil {
				return err
			}
			var length uint64
			length, err = qtls.ReadUvarint(decryptedReader)
			if err != nil {
				return err
			}
			index := len(decrypted) - decryptedReader.Len()
			fragments = append(fragments, qCryptoFragment{offset, length, decrypted[index : index+int(length)]})
			_, err = decryptedReader.Seek(int64(length), io.SeekCurrent)
			if err != nil {
				return err
			}
		case frameTypeConnectionClose:
			_, err = qtls.ReadUvarint(decryptedReader) // Error Code
			if err != nil {
				return err
			}
			_, err = qtls.ReadUvarint(decryptedReader) // Frame Type
			if err != nil {
				return err
			}
			var length uint64
			length, err = qtls.ReadUvarint(decryptedReader) // Reason Phrase Length
			if err != nil {
				return err
			}
			_, err = decryptedReader.Seek(int64(length), io.SeekCurrent) // Reason Phrase
			if err != nil {
				return err
			}
		default:
			return os.ErrInvalid
		}
	}
	if metadata.SniffContext != nil {
		fragments = append(fragments, metadata.SniffContext.([]qCryptoFragment)...)
		metadata.SniffContext = nil
	}
	var frameLen uint64
	for _, fragment := range fragments {
		frameLen += fragment.length
	}
	buffer := buf.NewSize(5 + int(frameLen))
	defer buffer.Release()
	buffer.WriteByte(0x16)
	binary.Write(buffer, binary.BigEndian, uint16(0x0303))
	binary.Write(buffer, binary.BigEndian, uint16(frameLen))
	var index uint64
	var length int
find:
	for {
		for _, fragment := range fragments {
			if fragment.offset == index {
				buffer.Write(fragment.payload)
				index = fragment.offset + fragment.length
				length++
				continue find
			}
		}
		break
	}
	metadata.Protocol = C.ProtocolQUIC
	fingerprint, err := ja3.Compute(buffer.Bytes())
	if err != nil {
		metadata.Protocol = C.ProtocolQUIC
		metadata.Client = C.ClientChromium
		metadata.SniffContext = fragments
		return E.Cause1(ErrNeedMoreData, err)
	}
	metadata.Domain = fingerprint.ServerName
	for metadata.Client == "" {
		if len(frameTypeList) == 1 {
			metadata.Client = C.ClientFirefox
			break
		}
		if frameTypeList[0] == frameTypeCrypto && isZero(frameTypeList[1:]) {
			if len(fingerprint.Versions) == 2 && fingerprint.Versions[0]&ja3.GreaseBitmask == 0x0A0A &&
				len(fingerprint.EllipticCurves) == 5 && fingerprint.EllipticCurves[0]&ja3.GreaseBitmask == 0x0A0A {
				metadata.Client = C.ClientSafari
				break
			}
			if len(fingerprint.CipherSuites) == 1 && fingerprint.CipherSuites[0] == tls.TLS_AES_256_GCM_SHA384 &&
				len(fingerprint.EllipticCurves) == 1 && fingerprint.EllipticCurves[0] == uint16(tls.X25519) &&
				len(fingerprint.SignatureAlgorithms) == 1 && fingerprint.SignatureAlgorithms[0] == uint16(tls.ECDSAWithP256AndSHA256) {
				metadata.Client = C.ClientSafari
				break
			}
		}

		if frameTypeList[len(frameTypeList)-1] == frameTypeCrypto && isZero(frameTypeList[:len(frameTypeList)-1]) {
			metadata.Client = C.ClientQUICGo
			break
		}

		if count(frameTypeList, frameTypeCrypto) > 1 || count(frameTypeList, frameTypePing) > 0 {
			if maybeUQUIC(fingerprint) {
				metadata.Client = C.ClientQUICGo
			} else {
				metadata.Client = C.ClientChromium
			}
			break
		}

		metadata.Client = C.ClientUnknown
		//nolint:staticcheck
		break
	}
	return nil
}

func isZero(slices []uint8) bool {
	for _, slice := range slices {
		if slice != 0 {
			return false
		}
	}
	return true
}

func count(slices []uint8, value uint8) int {
	var times int
	for _, slice := range slices {
		if slice == value {
			times++
		}
	}
	return times
}

type qCryptoFragment struct {
	offset  uint64
	length  uint64
	payload []byte
}
