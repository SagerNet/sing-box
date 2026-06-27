//go:build darwin

package local

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing-box/dns"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func (t *Transport) systemExchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	response, err := darwinLookupSystemDNS(ctx, question.Name, question.Qtype, question.Qclass)
	if err != nil {
		var rcodeError dns.RcodeError
		if errors.As(err, &rcodeError) {
			return dns.FixedResponseStatus(message, int(rcodeError)), nil
		}
		return nil, err
	}
	response.Id = message.Id
	response.Response = true
	response.RecursionAvailable = true
	return response, nil
}

// The mDNSResponder daemon speaks an undocumented binary protocol over a
// AF_UNIX SOCK_STREAM socket. The framing below is taken from the client
// stub of Apple's open-source mDNSResponder (mDNSShared/dnssd_ipc.h and
// dnssd_clientstub.c). All multi-byte fields are big-endian; for a one-shot
// query on a fresh, non-shared connection the request and every reply travel
// over the single connected stream (no SCM_RIGHTS, no return socket).
const (
	mdnsResponderSocketPath   = "/var/run/mDNSResponder"
	mdnsResponderSocketEnv    = "DNSSD_UDS_PATH"
	mdnsResponderVersion      = 1
	mdnsResponderHeaderLength = 28
	mdnsResponderQueryRequest = 8  // query_request
	mdnsResponderQueryReply   = 68 // query_reply_op

	mdnsResponderFlagMoreComing          = 0x1
	mdnsResponderFlagAdd                 = 0x2
	mdnsResponderFlagReturnIntermediates = 0x1000

	mdnsResponderErrNoError      = 0
	mdnsResponderErrNoSuchName   = -65538
	mdnsResponderErrNoSuchRecord = -65554
)

func darwinLookupSystemDNS(ctx context.Context, name string, qtype, qclass uint16) (*mDNS.Msg, error) {
	socketPath := cmp.Or(os.Getenv(mdnsResponderSocketEnv), mdnsResponderSocketPath)
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, E.Cause(err, "connect mDNSResponder")
	}
	defer conn.Close()
	stopCancel := context.AfterFunc(ctx, func() {
		conn.Close()
	})
	defer stopCancel()

	_, err = conn.Write(buildQueryRequest(name, qtype, qclass))
	if err != nil {
		return nil, contextError(ctx, E.Cause(err, "write mDNSResponder query"))
	}

	var status [4]byte
	_, err = io.ReadFull(conn, status[:])
	if err != nil {
		return nil, contextError(ctx, E.Cause(err, "read mDNSResponder status"))
	}
	statusCode := int32(binary.BigEndian.Uint32(status[:]))
	if statusCode != mdnsResponderErrNoError {
		return nil, darwinResolverError(name, statusCode)
	}

	var answers []mDNS.RR
	for {
		reply, replyErr := readReply(conn)
		if replyErr != nil {
			return nil, contextError(ctx, E.Cause(replyErr, "read mDNSResponder reply"))
		}
		if reply.errorCode != mdnsResponderErrNoError {
			if len(answers) > 0 {
				break
			}
			return nil, darwinResolverError(name, reply.errorCode)
		}
		if reply.flags&mdnsResponderFlagAdd != 0 && len(reply.rdata) > 0 {
			record, buildErr := buildResourceRecord(reply)
			if buildErr == nil {
				answers = append(answers, record)
			}
		}
		if reply.flags&mdnsResponderFlagMoreComing == 0 {
			break
		}
	}

	response := new(mDNS.Msg)
	response.Question = []mDNS.Question{{Name: mDNS.Fqdn(name), Qtype: qtype, Qclass: qclass}}
	response.Answer = answers
	return response, nil
}

func buildQueryRequest(name string, qtype, qclass uint16) []byte {
	payload := make([]byte, 0, 8+len(name)+1+4)
	payload = binary.BigEndian.AppendUint32(payload, mdnsResponderFlagReturnIntermediates)
	payload = binary.BigEndian.AppendUint32(payload, 0) // interfaceIndex
	payload = append(payload, name...)
	payload = append(payload, 0) // C string terminator
	payload = binary.BigEndian.AppendUint16(payload, qtype)
	payload = binary.BigEndian.AppendUint16(payload, qclass)

	message := make([]byte, mdnsResponderHeaderLength, mdnsResponderHeaderLength+len(payload))
	binary.BigEndian.PutUint32(message[0:], mdnsResponderVersion)
	binary.BigEndian.PutUint32(message[4:], uint32(len(payload)))
	binary.BigEndian.PutUint32(message[8:], 0) // ipc_flags
	binary.BigEndian.PutUint32(message[12:], mdnsResponderQueryRequest)
	// message[16:24] client_context and message[24:28] reg_index stay zero.
	return append(message, payload...)
}

type mdnsResponderReply struct {
	flags     uint32
	errorCode int32
	name      string
	rrtype    uint16
	rrclass   uint16
	ttl       uint32
	rdata     []byte
}

func readReply(conn net.Conn) (mdnsResponderReply, error) {
	var reply mdnsResponderReply
	var header [mdnsResponderHeaderLength]byte
	_, err := io.ReadFull(conn, header[:])
	if err != nil {
		return reply, err
	}
	dataLength := binary.BigEndian.Uint32(header[4:8])
	operation := binary.BigEndian.Uint32(header[12:16])
	if operation != mdnsResponderQueryReply {
		return reply, E.New("unexpected mDNSResponder reply op ", operation)
	}
	data := make([]byte, dataLength)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return reply, err
	}

	reader := replyReader{data: data}
	reply.flags = reader.uint32()
	reader.uint32() // interfaceIndex
	reply.errorCode = int32(reader.uint32())
	reply.name = reader.cString()
	reply.rrtype = reader.uint16()
	reply.rrclass = reader.uint16()
	rdlen := reader.uint16()
	reply.rdata = reader.bytes(int(rdlen))
	reply.ttl = reader.uint32()
	if reader.err != nil {
		return reply, reader.err
	}
	return reply, nil
}

func buildResourceRecord(reply mdnsResponderReply) (mDNS.RR, error) {
	name := mDNS.Fqdn(reply.name)
	nameBuffer := make([]byte, 256)
	offset, err := mDNS.PackDomainName(name, nameBuffer, 0, nil, false)
	if err != nil {
		return nil, err
	}
	record := make([]byte, 0, offset+10+len(reply.rdata))
	record = append(record, nameBuffer[:offset]...)
	record = binary.BigEndian.AppendUint16(record, reply.rrtype)
	record = binary.BigEndian.AppendUint16(record, reply.rrclass)
	record = binary.BigEndian.AppendUint32(record, reply.ttl)
	record = binary.BigEndian.AppendUint16(record, uint16(len(reply.rdata)))
	record = append(record, reply.rdata...)
	resourceRecord, _, err := mDNS.UnpackRR(record, 0)
	if err != nil {
		return nil, err
	}
	return resourceRecord, nil
}

// The daemon's NoSuchRecord conflates NXDOMAIN and NODATA, so it is reported as
// an empty NOERROR to avoid a false NXDOMAIN.
func darwinResolverError(name string, code int32) error {
	switch code {
	case mdnsResponderErrNoSuchRecord:
		return dns.RcodeSuccess
	case mdnsResponderErrNoSuchName:
		return dns.RcodeNameError
	default:
		return E.New("mDNSResponder query failed for ", name, ": error ", code)
	}
}

func contextError(ctx context.Context, err error) error {
	ctxErr := ctx.Err()
	if ctxErr != nil {
		return ctxErr
	}
	return err
}

type replyReader struct {
	data   []byte
	offset int
	err    error
}

func (r *replyReader) uint32() uint32 {
	if r.err != nil || r.offset+4 > len(r.data) {
		r.fail()
		return 0
	}
	value := binary.BigEndian.Uint32(r.data[r.offset:])
	r.offset += 4
	return value
}

func (r *replyReader) uint16() uint16 {
	if r.err != nil || r.offset+2 > len(r.data) {
		r.fail()
		return 0
	}
	value := binary.BigEndian.Uint16(r.data[r.offset:])
	r.offset += 2
	return value
}

func (r *replyReader) cString() string {
	if r.err != nil {
		return ""
	}
	end := r.offset
	for end < len(r.data) && r.data[end] != 0 {
		end++
	}
	if end >= len(r.data) {
		r.fail()
		return ""
	}
	value := string(r.data[r.offset:end])
	r.offset = end + 1
	return value
}

func (r *replyReader) bytes(length int) []byte {
	if r.err != nil || length < 0 || r.offset+length > len(r.data) {
		r.fail()
		return nil
	}
	value := r.data[r.offset : r.offset+length]
	r.offset += length
	return value
}

func (r *replyReader) fail() {
	if r.err == nil {
		r.err = E.New("truncated mDNSResponder reply")
	}
}
