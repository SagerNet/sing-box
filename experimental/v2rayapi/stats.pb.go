package v2rayapi

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

type GetStatsRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Name of the stat counter.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Whether or not to reset the counter to fetching its value.
	Reset_        bool `protobuf:"varint,2,opt,name=reset,proto3" json:"reset,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetStatsRequest) Reset() {
	*x = GetStatsRequest{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetStatsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatsRequest) ProtoMessage() {}

func (x *GetStatsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatsRequest.ProtoReflect.Descriptor instead.
func (*GetStatsRequest) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{0}
}

func (x *GetStatsRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *GetStatsRequest) GetReset_() bool {
	if x != nil {
		return x.Reset_
	}
	return false
}

type Stat struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value         int64                  `protobuf:"varint,2,opt,name=value,proto3" json:"value,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Stat) Reset() {
	*x = Stat{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Stat) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Stat) ProtoMessage() {}

func (x *Stat) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Stat.ProtoReflect.Descriptor instead.
func (*Stat) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{1}
}

func (x *Stat) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Stat) GetValue() int64 {
	if x != nil {
		return x.Value
	}
	return 0
}

type GetStatsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Stat          *Stat                  `protobuf:"bytes,1,opt,name=stat,proto3" json:"stat,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetStatsResponse) Reset() {
	*x = GetStatsResponse{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetStatsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatsResponse) ProtoMessage() {}

func (x *GetStatsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatsResponse.ProtoReflect.Descriptor instead.
func (*GetStatsResponse) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{2}
}

func (x *GetStatsResponse) GetStat() *Stat {
	if x != nil {
		return x.Stat
	}
	return nil
}

type QueryStatsRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Deprecated, use Patterns instead
	Pattern       string   `protobuf:"bytes,1,opt,name=pattern,proto3" json:"pattern,omitempty"`
	Reset_        bool     `protobuf:"varint,2,opt,name=reset,proto3" json:"reset,omitempty"`
	Patterns      []string `protobuf:"bytes,3,rep,name=patterns,proto3" json:"patterns,omitempty"`
	Regexp        bool     `protobuf:"varint,4,opt,name=regexp,proto3" json:"regexp,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QueryStatsRequest) Reset() {
	*x = QueryStatsRequest{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QueryStatsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryStatsRequest) ProtoMessage() {}

func (x *QueryStatsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryStatsRequest.ProtoReflect.Descriptor instead.
func (*QueryStatsRequest) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{3}
}

func (x *QueryStatsRequest) GetPattern() string {
	if x != nil {
		return x.Pattern
	}
	return ""
}

func (x *QueryStatsRequest) GetReset_() bool {
	if x != nil {
		return x.Reset_
	}
	return false
}

func (x *QueryStatsRequest) GetPatterns() []string {
	if x != nil {
		return x.Patterns
	}
	return nil
}

func (x *QueryStatsRequest) GetRegexp() bool {
	if x != nil {
		return x.Regexp
	}
	return false
}

type QueryStatsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Stat          []*Stat                `protobuf:"bytes,1,rep,name=stat,proto3" json:"stat,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QueryStatsResponse) Reset() {
	*x = QueryStatsResponse{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QueryStatsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryStatsResponse) ProtoMessage() {}

func (x *QueryStatsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryStatsResponse.ProtoReflect.Descriptor instead.
func (*QueryStatsResponse) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{4}
}

func (x *QueryStatsResponse) GetStat() []*Stat {
	if x != nil {
		return x.Stat
	}
	return nil
}

type SysStatsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SysStatsRequest) Reset() {
	*x = SysStatsRequest{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SysStatsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SysStatsRequest) ProtoMessage() {}

func (x *SysStatsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SysStatsRequest.ProtoReflect.Descriptor instead.
func (*SysStatsRequest) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{5}
}

type SysStatsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	NumGoroutine  uint32                 `protobuf:"varint,1,opt,name=NumGoroutine,proto3" json:"NumGoroutine,omitempty"`
	NumGC         uint32                 `protobuf:"varint,2,opt,name=NumGC,proto3" json:"NumGC,omitempty"`
	Alloc         uint64                 `protobuf:"varint,3,opt,name=Alloc,proto3" json:"Alloc,omitempty"`
	TotalAlloc    uint64                 `protobuf:"varint,4,opt,name=TotalAlloc,proto3" json:"TotalAlloc,omitempty"`
	Sys           uint64                 `protobuf:"varint,5,opt,name=Sys,proto3" json:"Sys,omitempty"`
	Mallocs       uint64                 `protobuf:"varint,6,opt,name=Mallocs,proto3" json:"Mallocs,omitempty"`
	Frees         uint64                 `protobuf:"varint,7,opt,name=Frees,proto3" json:"Frees,omitempty"`
	LiveObjects   uint64                 `protobuf:"varint,8,opt,name=LiveObjects,proto3" json:"LiveObjects,omitempty"`
	PauseTotalNs  uint64                 `protobuf:"varint,9,opt,name=PauseTotalNs,proto3" json:"PauseTotalNs,omitempty"`
	Uptime        uint32                 `protobuf:"varint,10,opt,name=Uptime,proto3" json:"Uptime,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SysStatsResponse) Reset() {
	*x = SysStatsResponse{}
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SysStatsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SysStatsResponse) ProtoMessage() {}

func (x *SysStatsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_v2rayapi_stats_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SysStatsResponse.ProtoReflect.Descriptor instead.
func (*SysStatsResponse) Descriptor() ([]byte, []int) {
	return file_experimental_v2rayapi_stats_proto_rawDescGZIP(), []int{6}
}

func (x *SysStatsResponse) GetNumGoroutine() uint32 {
	if x != nil {
		return x.NumGoroutine
	}
	return 0
}

func (x *SysStatsResponse) GetNumGC() uint32 {
	if x != nil {
		return x.NumGC
	}
	return 0
}

func (x *SysStatsResponse) GetAlloc() uint64 {
	if x != nil {
		return x.Alloc
	}
	return 0
}

func (x *SysStatsResponse) GetTotalAlloc() uint64 {
	if x != nil {
		return x.TotalAlloc
	}
	return 0
}

func (x *SysStatsResponse) GetSys() uint64 {
	if x != nil {
		return x.Sys
	}
	return 0
}

func (x *SysStatsResponse) GetMallocs() uint64 {
	if x != nil {
		return x.Mallocs
	}
	return 0
}

func (x *SysStatsResponse) GetFrees() uint64 {
	if x != nil {
		return x.Frees
	}
	return 0
}

func (x *SysStatsResponse) GetLiveObjects() uint64 {
	if x != nil {
		return x.LiveObjects
	}
	return 0
}

func (x *SysStatsResponse) GetPauseTotalNs() uint64 {
	if x != nil {
		return x.PauseTotalNs
	}
	return 0
}

func (x *SysStatsResponse) GetUptime() uint32 {
	if x != nil {
		return x.Uptime
	}
	return 0
}

var File_experimental_v2rayapi_stats_proto protoreflect.FileDescriptor

const file_experimental_v2rayapi_stats_proto_rawDesc = "" +
	"\n" +
	"!experimental/v2rayapi/stats.proto\x12\x15experimental.v2rayapi\";\n" +
	"\x0fGetStatsRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x14\n" +
	"\x05reset\x18\x02 \x01(\bR\x05reset\"0\n" +
	"\x04Stat\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x14\n" +
	"\x05value\x18\x02 \x01(\x03R\x05value\"C\n" +
	"\x10GetStatsResponse\x12/\n" +
	"\x04stat\x18\x01 \x01(\v2\x1b.experimental.v2rayapi.StatR\x04stat\"w\n" +
	"\x11QueryStatsRequest\x12\x18\n" +
	"\apattern\x18\x01 \x01(\tR\apattern\x12\x14\n" +
	"\x05reset\x18\x02 \x01(\bR\x05reset\x12\x1a\n" +
	"\bpatterns\x18\x03 \x03(\tR\bpatterns\x12\x16\n" +
	"\x06regexp\x18\x04 \x01(\bR\x06regexp\"E\n" +
	"\x12QueryStatsResponse\x12/\n" +
	"\x04stat\x18\x01 \x03(\v2\x1b.experimental.v2rayapi.StatR\x04stat\"\x11\n" +
	"\x0fSysStatsRequest\"\xa2\x02\n" +
	"\x10SysStatsResponse\x12\"\n" +
	"\fNumGoroutine\x18\x01 \x01(\rR\fNumGoroutine\x12\x14\n" +
	"\x05NumGC\x18\x02 \x01(\rR\x05NumGC\x12\x14\n" +
	"\x05Alloc\x18\x03 \x01(\x04R\x05Alloc\x12\x1e\n" +
	"\n" +
	"TotalAlloc\x18\x04 \x01(\x04R\n" +
	"TotalAlloc\x12\x10\n" +
	"\x03Sys\x18\x05 \x01(\x04R\x03Sys\x12\x18\n" +
	"\aMallocs\x18\x06 \x01(\x04R\aMallocs\x12\x14\n" +
	"\x05Frees\x18\a \x01(\x04R\x05Frees\x12 \n" +
	"\vLiveObjects\x18\b \x01(\x04R\vLiveObjects\x12\"\n" +
	"\fPauseTotalNs\x18\t \x01(\x04R\fPauseTotalNs\x12\x16\n" +
	"\x06Uptime\x18\n" +
	" \x01(\rR\x06Uptime2\xb4\x02\n" +
	"\fStatsService\x12]\n" +
	"\bGetStats\x12&.experimental.v2rayapi.GetStatsRequest\x1a'.experimental.v2rayapi.GetStatsResponse\"\x00\x12c\n" +
	"\n" +
	"QueryStats\x12(.experimental.v2rayapi.QueryStatsRequest\x1a).experimental.v2rayapi.QueryStatsResponse\"\x00\x12`\n" +
	"\vGetSysStats\x12&.experimental.v2rayapi.SysStatsRequest\x1a'.experimental.v2rayapi.SysStatsResponse\"\x00B4Z2github.com/sagernet/sing-box/experimental/v2rayapib\x06proto3"

var (
	file_experimental_v2rayapi_stats_proto_rawDescOnce sync.Once
	file_experimental_v2rayapi_stats_proto_rawDescData []byte
)

func file_experimental_v2rayapi_stats_proto_rawDescGZIP() []byte {
	file_experimental_v2rayapi_stats_proto_rawDescOnce.Do(func() {
		file_experimental_v2rayapi_stats_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_experimental_v2rayapi_stats_proto_rawDesc), len(file_experimental_v2rayapi_stats_proto_rawDesc)))
	})
	return file_experimental_v2rayapi_stats_proto_rawDescData
}

var (
	file_experimental_v2rayapi_stats_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
	file_experimental_v2rayapi_stats_proto_goTypes  = []any{
		(*GetStatsRequest)(nil),    // 0: experimental.v2rayapi.GetStatsRequest
		(*Stat)(nil),               // 1: experimental.v2rayapi.Stat
		(*GetStatsResponse)(nil),   // 2: experimental.v2rayapi.GetStatsResponse
		(*QueryStatsRequest)(nil),  // 3: experimental.v2rayapi.QueryStatsRequest
		(*QueryStatsResponse)(nil), // 4: experimental.v2rayapi.QueryStatsResponse
		(*SysStatsRequest)(nil),    // 5: experimental.v2rayapi.SysStatsRequest
		(*SysStatsResponse)(nil),   // 6: experimental.v2rayapi.SysStatsResponse
	}
)

var file_experimental_v2rayapi_stats_proto_depIdxs = []int32{
	1, // 0: experimental.v2rayapi.GetStatsResponse.stat:type_name -> experimental.v2rayapi.Stat
	1, // 1: experimental.v2rayapi.QueryStatsResponse.stat:type_name -> experimental.v2rayapi.Stat
	0, // 2: experimental.v2rayapi.StatsService.GetStats:input_type -> experimental.v2rayapi.GetStatsRequest
	3, // 3: experimental.v2rayapi.StatsService.QueryStats:input_type -> experimental.v2rayapi.QueryStatsRequest
	5, // 4: experimental.v2rayapi.StatsService.GetSysStats:input_type -> experimental.v2rayapi.SysStatsRequest
	2, // 5: experimental.v2rayapi.StatsService.GetStats:output_type -> experimental.v2rayapi.GetStatsResponse
	4, // 6: experimental.v2rayapi.StatsService.QueryStats:output_type -> experimental.v2rayapi.QueryStatsResponse
	6, // 7: experimental.v2rayapi.StatsService.GetSysStats:output_type -> experimental.v2rayapi.SysStatsResponse
	5, // [5:8] is the sub-list for method output_type
	2, // [2:5] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_experimental_v2rayapi_stats_proto_init() }
func file_experimental_v2rayapi_stats_proto_init() {
	if File_experimental_v2rayapi_stats_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_experimental_v2rayapi_stats_proto_rawDesc), len(file_experimental_v2rayapi_stats_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_experimental_v2rayapi_stats_proto_goTypes,
		DependencyIndexes: file_experimental_v2rayapi_stats_proto_depIdxs,
		MessageInfos:      file_experimental_v2rayapi_stats_proto_msgTypes,
	}.Build()
	File_experimental_v2rayapi_stats_proto = out.File
	file_experimental_v2rayapi_stats_proto_goTypes = nil
	file_experimental_v2rayapi_stats_proto_depIdxs = nil
}
