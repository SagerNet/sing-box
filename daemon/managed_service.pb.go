package daemon

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DebugCrashRequest_Type int32

const (
	DebugCrashRequest_GO     DebugCrashRequest_Type = 0
	DebugCrashRequest_NATIVE DebugCrashRequest_Type = 1
)

// Enum value maps for DebugCrashRequest_Type.
var (
	DebugCrashRequest_Type_name = map[int32]string{
		0: "GO",
		1: "NATIVE",
	}
	DebugCrashRequest_Type_value = map[string]int32{
		"GO":     0,
		"NATIVE": 1,
	}
)

func (x DebugCrashRequest_Type) Enum() *DebugCrashRequest_Type {
	p := new(DebugCrashRequest_Type)
	*p = x
	return p
}

func (x DebugCrashRequest_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DebugCrashRequest_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_daemon_managed_service_proto_enumTypes[0].Descriptor()
}

func (DebugCrashRequest_Type) Type() protoreflect.EnumType {
	return &file_daemon_managed_service_proto_enumTypes[0]
}

func (x DebugCrashRequest_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DebugCrashRequest_Type.Descriptor instead.
func (DebugCrashRequest_Type) EnumDescriptor() ([]byte, []int) {
	return file_daemon_managed_service_proto_rawDescGZIP(), []int{2, 0}
}

type SystemProxyStatus struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Available     bool                   `protobuf:"varint,1,opt,name=available,proto3" json:"available,omitempty"`
	Enabled       bool                   `protobuf:"varint,2,opt,name=enabled,proto3" json:"enabled,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SystemProxyStatus) Reset() {
	*x = SystemProxyStatus{}
	mi := &file_daemon_managed_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SystemProxyStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemProxyStatus) ProtoMessage() {}

func (x *SystemProxyStatus) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_managed_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemProxyStatus.ProtoReflect.Descriptor instead.
func (*SystemProxyStatus) Descriptor() ([]byte, []int) {
	return file_daemon_managed_service_proto_rawDescGZIP(), []int{0}
}

func (x *SystemProxyStatus) GetAvailable() bool {
	if x != nil {
		return x.Available
	}
	return false
}

func (x *SystemProxyStatus) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

type SetSystemProxyEnabledRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Enabled       bool                   `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SetSystemProxyEnabledRequest) Reset() {
	*x = SetSystemProxyEnabledRequest{}
	mi := &file_daemon_managed_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SetSystemProxyEnabledRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetSystemProxyEnabledRequest) ProtoMessage() {}

func (x *SetSystemProxyEnabledRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_managed_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetSystemProxyEnabledRequest.ProtoReflect.Descriptor instead.
func (*SetSystemProxyEnabledRequest) Descriptor() ([]byte, []int) {
	return file_daemon_managed_service_proto_rawDescGZIP(), []int{1}
}

func (x *SetSystemProxyEnabledRequest) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

type DebugCrashRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Type          DebugCrashRequest_Type `protobuf:"varint,1,opt,name=type,proto3,enum=daemon.DebugCrashRequest_Type" json:"type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DebugCrashRequest) Reset() {
	*x = DebugCrashRequest{}
	mi := &file_daemon_managed_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DebugCrashRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DebugCrashRequest) ProtoMessage() {}

func (x *DebugCrashRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_managed_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DebugCrashRequest.ProtoReflect.Descriptor instead.
func (*DebugCrashRequest) Descriptor() ([]byte, []int) {
	return file_daemon_managed_service_proto_rawDescGZIP(), []int{2}
}

func (x *DebugCrashRequest) GetType() DebugCrashRequest_Type {
	if x != nil {
		return x.Type
	}
	return DebugCrashRequest_GO
}

var File_daemon_managed_service_proto protoreflect.FileDescriptor

const file_daemon_managed_service_proto_rawDesc = "" +
	"\n" +
	"\x1cdaemon/managed_service.proto\x12\x06daemon\x1a\x1bgoogle/protobuf/empty.proto\"K\n" +
	"\x11SystemProxyStatus\x12\x1c\n" +
	"\tavailable\x18\x01 \x01(\bR\tavailable\x12\x18\n" +
	"\aenabled\x18\x02 \x01(\bR\aenabled\"8\n" +
	"\x1cSetSystemProxyEnabledRequest\x12\x18\n" +
	"\aenabled\x18\x01 \x01(\bR\aenabled\"c\n" +
	"\x11DebugCrashRequest\x122\n" +
	"\x04type\x18\x01 \x01(\x0e2\x1e.daemon.DebugCrashRequest.TypeR\x04type\"\x1a\n" +
	"\x04Type\x12\x06\n" +
	"\x02GO\x10\x00\x12\n" +
	"\n" +
	"\x06NATIVE\x10\x012\x80\x03\n" +
	"\x0eManagedService\x12=\n" +
	"\vStopService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\x12?\n" +
	"\rReloadService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\x12K\n" +
	"\x14GetSystemProxyStatus\x12\x16.google.protobuf.Empty\x1a\x19.daemon.SystemProxyStatus\"\x00\x12W\n" +
	"\x15SetSystemProxyEnabled\x12$.daemon.SetSystemProxyEnabledRequest\x1a\x16.google.protobuf.Empty\"\x00\x12H\n" +
	"\x11TriggerDebugCrash\x12\x19.daemon.DebugCrashRequest\x1a\x16.google.protobuf.Empty\"\x00B%Z#github.com/sagernet/sing-box/daemonb\x06proto3"

var (
	file_daemon_managed_service_proto_rawDescOnce sync.Once
	file_daemon_managed_service_proto_rawDescData []byte
)

func file_daemon_managed_service_proto_rawDescGZIP() []byte {
	file_daemon_managed_service_proto_rawDescOnce.Do(func() {
		file_daemon_managed_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_daemon_managed_service_proto_rawDesc), len(file_daemon_managed_service_proto_rawDesc)))
	})
	return file_daemon_managed_service_proto_rawDescData
}

var (
	file_daemon_managed_service_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
	file_daemon_managed_service_proto_msgTypes  = make([]protoimpl.MessageInfo, 3)
	file_daemon_managed_service_proto_goTypes   = []any{
		(DebugCrashRequest_Type)(0),          // 0: daemon.DebugCrashRequest.Type
		(*SystemProxyStatus)(nil),            // 1: daemon.SystemProxyStatus
		(*SetSystemProxyEnabledRequest)(nil), // 2: daemon.SetSystemProxyEnabledRequest
		(*DebugCrashRequest)(nil),            // 3: daemon.DebugCrashRequest
		(*emptypb.Empty)(nil),                // 4: google.protobuf.Empty
	}
)

var file_daemon_managed_service_proto_depIdxs = []int32{
	0, // 0: daemon.DebugCrashRequest.type:type_name -> daemon.DebugCrashRequest.Type
	4, // 1: daemon.ManagedService.StopService:input_type -> google.protobuf.Empty
	4, // 2: daemon.ManagedService.ReloadService:input_type -> google.protobuf.Empty
	4, // 3: daemon.ManagedService.GetSystemProxyStatus:input_type -> google.protobuf.Empty
	2, // 4: daemon.ManagedService.SetSystemProxyEnabled:input_type -> daemon.SetSystemProxyEnabledRequest
	3, // 5: daemon.ManagedService.TriggerDebugCrash:input_type -> daemon.DebugCrashRequest
	4, // 6: daemon.ManagedService.StopService:output_type -> google.protobuf.Empty
	4, // 7: daemon.ManagedService.ReloadService:output_type -> google.protobuf.Empty
	1, // 8: daemon.ManagedService.GetSystemProxyStatus:output_type -> daemon.SystemProxyStatus
	4, // 9: daemon.ManagedService.SetSystemProxyEnabled:output_type -> google.protobuf.Empty
	4, // 10: daemon.ManagedService.TriggerDebugCrash:output_type -> google.protobuf.Empty
	6, // [6:11] is the sub-list for method output_type
	1, // [1:6] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_daemon_managed_service_proto_init() }
func file_daemon_managed_service_proto_init() {
	if File_daemon_managed_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_daemon_managed_service_proto_rawDesc), len(file_daemon_managed_service_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_daemon_managed_service_proto_goTypes,
		DependencyIndexes: file_daemon_managed_service_proto_depIdxs,
		EnumInfos:         file_daemon_managed_service_proto_enumTypes,
		MessageInfos:      file_daemon_managed_service_proto_msgTypes,
	}.Build()
	File_daemon_managed_service_proto = out.File
	file_daemon_managed_service_proto_goTypes = nil
	file_daemon_managed_service_proto_depIdxs = nil
}
