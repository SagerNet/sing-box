package v2raygrpc

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Hunk struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          []byte                 `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hunk) Reset() {
	*x = Hunk{}
	mi := &file_transport_v2raygrpc_stream_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hunk) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hunk) ProtoMessage() {}

func (x *Hunk) ProtoReflect() protoreflect.Message {
	mi := &file_transport_v2raygrpc_stream_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hunk.ProtoReflect.Descriptor instead.
func (*Hunk) Descriptor() ([]byte, []int) {
	return file_transport_v2raygrpc_stream_proto_rawDescGZIP(), []int{0}
}

func (x *Hunk) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_transport_v2raygrpc_stream_proto protoreflect.FileDescriptor

const file_transport_v2raygrpc_stream_proto_rawDesc = "" +
	"\n" +
	" transport/v2raygrpc/stream.proto\x12\x13transport.v2raygrpc\"\x1a\n" +
	"\x04Hunk\x12\x12\n" +
	"\x04data\x18\x01 \x01(\fR\x04data2M\n" +
	"\n" +
	"GunService\x12?\n" +
	"\x03Tun\x12\x19.transport.v2raygrpc.Hunk\x1a\x19.transport.v2raygrpc.Hunk(\x010\x01B2Z0github.com/sagernet/sing-box/transport/v2raygrpcb\x06proto3"

var (
	file_transport_v2raygrpc_stream_proto_rawDescOnce sync.Once
	file_transport_v2raygrpc_stream_proto_rawDescData []byte
)

func file_transport_v2raygrpc_stream_proto_rawDescGZIP() []byte {
	file_transport_v2raygrpc_stream_proto_rawDescOnce.Do(func() {
		file_transport_v2raygrpc_stream_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_transport_v2raygrpc_stream_proto_rawDesc), len(file_transport_v2raygrpc_stream_proto_rawDesc)))
	})
	return file_transport_v2raygrpc_stream_proto_rawDescData
}

var (
	file_transport_v2raygrpc_stream_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
	file_transport_v2raygrpc_stream_proto_goTypes  = []any{
		(*Hunk)(nil), // 0: transport.v2raygrpc.Hunk
	}
)

var file_transport_v2raygrpc_stream_proto_depIdxs = []int32{
	0, // 0: transport.v2raygrpc.GunService.Tun:input_type -> transport.v2raygrpc.Hunk
	0, // 1: transport.v2raygrpc.GunService.Tun:output_type -> transport.v2raygrpc.Hunk
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_transport_v2raygrpc_stream_proto_init() }
func file_transport_v2raygrpc_stream_proto_init() {
	if File_transport_v2raygrpc_stream_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_transport_v2raygrpc_stream_proto_rawDesc), len(file_transport_v2raygrpc_stream_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_transport_v2raygrpc_stream_proto_goTypes,
		DependencyIndexes: file_transport_v2raygrpc_stream_proto_depIdxs,
		MessageInfos:      file_transport_v2raygrpc_stream_proto_msgTypes,
	}.Build()
	File_transport_v2raygrpc_stream_proto = out.File
	file_transport_v2raygrpc_stream_proto_goTypes = nil
	file_transport_v2raygrpc_stream_proto_depIdxs = nil
}
