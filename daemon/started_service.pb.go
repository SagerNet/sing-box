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

type LogLevel int32

const (
	LogLevel_PANIC LogLevel = 0
	LogLevel_FATAL LogLevel = 1
	LogLevel_ERROR LogLevel = 2
	LogLevel_WARN  LogLevel = 3
	LogLevel_INFO  LogLevel = 4
	LogLevel_DEBUG LogLevel = 5
	LogLevel_TRACE LogLevel = 6
)

// Enum value maps for LogLevel.
var (
	LogLevel_name = map[int32]string{
		0: "PANIC",
		1: "FATAL",
		2: "ERROR",
		3: "WARN",
		4: "INFO",
		5: "DEBUG",
		6: "TRACE",
	}
	LogLevel_value = map[string]int32{
		"PANIC": 0,
		"FATAL": 1,
		"ERROR": 2,
		"WARN":  3,
		"INFO":  4,
		"DEBUG": 5,
		"TRACE": 6,
	}
)

func (x LogLevel) Enum() *LogLevel {
	p := new(LogLevel)
	*p = x
	return p
}

func (x LogLevel) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogLevel) Descriptor() protoreflect.EnumDescriptor {
	return file_daemon_started_service_proto_enumTypes[0].Descriptor()
}

func (LogLevel) Type() protoreflect.EnumType {
	return &file_daemon_started_service_proto_enumTypes[0]
}

func (x LogLevel) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogLevel.Descriptor instead.
func (LogLevel) EnumDescriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{0}
}

type ConnectionFilter int32

const (
	ConnectionFilter_ALL    ConnectionFilter = 0
	ConnectionFilter_ACTIVE ConnectionFilter = 1
	ConnectionFilter_CLOSED ConnectionFilter = 2
)

// Enum value maps for ConnectionFilter.
var (
	ConnectionFilter_name = map[int32]string{
		0: "ALL",
		1: "ACTIVE",
		2: "CLOSED",
	}
	ConnectionFilter_value = map[string]int32{
		"ALL":    0,
		"ACTIVE": 1,
		"CLOSED": 2,
	}
)

func (x ConnectionFilter) Enum() *ConnectionFilter {
	p := new(ConnectionFilter)
	*p = x
	return p
}

func (x ConnectionFilter) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ConnectionFilter) Descriptor() protoreflect.EnumDescriptor {
	return file_daemon_started_service_proto_enumTypes[1].Descriptor()
}

func (ConnectionFilter) Type() protoreflect.EnumType {
	return &file_daemon_started_service_proto_enumTypes[1]
}

func (x ConnectionFilter) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ConnectionFilter.Descriptor instead.
func (ConnectionFilter) EnumDescriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{1}
}

type ConnectionSortBy int32

const (
	ConnectionSortBy_DATE          ConnectionSortBy = 0
	ConnectionSortBy_TRAFFIC       ConnectionSortBy = 1
	ConnectionSortBy_TOTAL_TRAFFIC ConnectionSortBy = 2
)

// Enum value maps for ConnectionSortBy.
var (
	ConnectionSortBy_name = map[int32]string{
		0: "DATE",
		1: "TRAFFIC",
		2: "TOTAL_TRAFFIC",
	}
	ConnectionSortBy_value = map[string]int32{
		"DATE":          0,
		"TRAFFIC":       1,
		"TOTAL_TRAFFIC": 2,
	}
)

func (x ConnectionSortBy) Enum() *ConnectionSortBy {
	p := new(ConnectionSortBy)
	*p = x
	return p
}

func (x ConnectionSortBy) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ConnectionSortBy) Descriptor() protoreflect.EnumDescriptor {
	return file_daemon_started_service_proto_enumTypes[2].Descriptor()
}

func (ConnectionSortBy) Type() protoreflect.EnumType {
	return &file_daemon_started_service_proto_enumTypes[2]
}

func (x ConnectionSortBy) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ConnectionSortBy.Descriptor instead.
func (ConnectionSortBy) EnumDescriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{2}
}

type ServiceStatus_Type int32

const (
	ServiceStatus_IDLE     ServiceStatus_Type = 0
	ServiceStatus_STARTING ServiceStatus_Type = 1
	ServiceStatus_STARTED  ServiceStatus_Type = 2
	ServiceStatus_STOPPING ServiceStatus_Type = 3
	ServiceStatus_FATAL    ServiceStatus_Type = 4
)

// Enum value maps for ServiceStatus_Type.
var (
	ServiceStatus_Type_name = map[int32]string{
		0: "IDLE",
		1: "STARTING",
		2: "STARTED",
		3: "STOPPING",
		4: "FATAL",
	}
	ServiceStatus_Type_value = map[string]int32{
		"IDLE":     0,
		"STARTING": 1,
		"STARTED":  2,
		"STOPPING": 3,
		"FATAL":    4,
	}
)

func (x ServiceStatus_Type) Enum() *ServiceStatus_Type {
	p := new(ServiceStatus_Type)
	*p = x
	return p
}

func (x ServiceStatus_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ServiceStatus_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_daemon_started_service_proto_enumTypes[3].Descriptor()
}

func (ServiceStatus_Type) Type() protoreflect.EnumType {
	return &file_daemon_started_service_proto_enumTypes[3]
}

func (x ServiceStatus_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ServiceStatus_Type.Descriptor instead.
func (ServiceStatus_Type) EnumDescriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{0, 0}
}

type ServiceStatus struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        ServiceStatus_Type     `protobuf:"varint,1,opt,name=status,proto3,enum=daemon.ServiceStatus_Type" json:"status,omitempty"`
	ErrorMessage  string                 `protobuf:"bytes,2,opt,name=errorMessage,proto3" json:"errorMessage,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServiceStatus) Reset() {
	*x = ServiceStatus{}
	mi := &file_daemon_started_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServiceStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServiceStatus) ProtoMessage() {}

func (x *ServiceStatus) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServiceStatus.ProtoReflect.Descriptor instead.
func (*ServiceStatus) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{0}
}

func (x *ServiceStatus) GetStatus() ServiceStatus_Type {
	if x != nil {
		return x.Status
	}
	return ServiceStatus_IDLE
}

func (x *ServiceStatus) GetErrorMessage() string {
	if x != nil {
		return x.ErrorMessage
	}
	return ""
}

type ReloadServiceRequest struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	NewProfileContent string                 `protobuf:"bytes,1,opt,name=newProfileContent,proto3" json:"newProfileContent,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ReloadServiceRequest) Reset() {
	*x = ReloadServiceRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ReloadServiceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReloadServiceRequest) ProtoMessage() {}

func (x *ReloadServiceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReloadServiceRequest.ProtoReflect.Descriptor instead.
func (*ReloadServiceRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{1}
}

func (x *ReloadServiceRequest) GetNewProfileContent() string {
	if x != nil {
		return x.NewProfileContent
	}
	return ""
}

type SubscribeStatusRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Interval      int64                  `protobuf:"varint,1,opt,name=interval,proto3" json:"interval,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SubscribeStatusRequest) Reset() {
	*x = SubscribeStatusRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeStatusRequest) ProtoMessage() {}

func (x *SubscribeStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeStatusRequest.ProtoReflect.Descriptor instead.
func (*SubscribeStatusRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{2}
}

func (x *SubscribeStatusRequest) GetInterval() int64 {
	if x != nil {
		return x.Interval
	}
	return 0
}

type Log struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Messages      []*Log_Message         `protobuf:"bytes,1,rep,name=messages,proto3" json:"messages,omitempty"`
	Reset_        bool                   `protobuf:"varint,2,opt,name=reset,proto3" json:"reset,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Log) Reset() {
	*x = Log{}
	mi := &file_daemon_started_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Log) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Log) ProtoMessage() {}

func (x *Log) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Log.ProtoReflect.Descriptor instead.
func (*Log) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{3}
}

func (x *Log) GetMessages() []*Log_Message {
	if x != nil {
		return x.Messages
	}
	return nil
}

func (x *Log) GetReset_() bool {
	if x != nil {
		return x.Reset_
	}
	return false
}

type DefaultLogLevel struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Level         LogLevel               `protobuf:"varint,1,opt,name=level,proto3,enum=daemon.LogLevel" json:"level,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DefaultLogLevel) Reset() {
	*x = DefaultLogLevel{}
	mi := &file_daemon_started_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DefaultLogLevel) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DefaultLogLevel) ProtoMessage() {}

func (x *DefaultLogLevel) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DefaultLogLevel.ProtoReflect.Descriptor instead.
func (*DefaultLogLevel) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{4}
}

func (x *DefaultLogLevel) GetLevel() LogLevel {
	if x != nil {
		return x.Level
	}
	return LogLevel_PANIC
}

type Status struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	Memory           uint64                 `protobuf:"varint,1,opt,name=memory,proto3" json:"memory,omitempty"`
	Goroutines       int32                  `protobuf:"varint,2,opt,name=goroutines,proto3" json:"goroutines,omitempty"`
	ConnectionsIn    int32                  `protobuf:"varint,3,opt,name=connectionsIn,proto3" json:"connectionsIn,omitempty"`
	ConnectionsOut   int32                  `protobuf:"varint,4,opt,name=connectionsOut,proto3" json:"connectionsOut,omitempty"`
	TrafficAvailable bool                   `protobuf:"varint,5,opt,name=trafficAvailable,proto3" json:"trafficAvailable,omitempty"`
	Uplink           int64                  `protobuf:"varint,6,opt,name=uplink,proto3" json:"uplink,omitempty"`
	Downlink         int64                  `protobuf:"varint,7,opt,name=downlink,proto3" json:"downlink,omitempty"`
	UplinkTotal      int64                  `protobuf:"varint,8,opt,name=uplinkTotal,proto3" json:"uplinkTotal,omitempty"`
	DownlinkTotal    int64                  `protobuf:"varint,9,opt,name=downlinkTotal,proto3" json:"downlinkTotal,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Status) Reset() {
	*x = Status{}
	mi := &file_daemon_started_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status.ProtoReflect.Descriptor instead.
func (*Status) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{5}
}

func (x *Status) GetMemory() uint64 {
	if x != nil {
		return x.Memory
	}
	return 0
}

func (x *Status) GetGoroutines() int32 {
	if x != nil {
		return x.Goroutines
	}
	return 0
}

func (x *Status) GetConnectionsIn() int32 {
	if x != nil {
		return x.ConnectionsIn
	}
	return 0
}

func (x *Status) GetConnectionsOut() int32 {
	if x != nil {
		return x.ConnectionsOut
	}
	return 0
}

func (x *Status) GetTrafficAvailable() bool {
	if x != nil {
		return x.TrafficAvailable
	}
	return false
}

func (x *Status) GetUplink() int64 {
	if x != nil {
		return x.Uplink
	}
	return 0
}

func (x *Status) GetDownlink() int64 {
	if x != nil {
		return x.Downlink
	}
	return 0
}

func (x *Status) GetUplinkTotal() int64 {
	if x != nil {
		return x.UplinkTotal
	}
	return 0
}

func (x *Status) GetDownlinkTotal() int64 {
	if x != nil {
		return x.DownlinkTotal
	}
	return 0
}

type Groups struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Group         []*Group               `protobuf:"bytes,1,rep,name=group,proto3" json:"group,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Groups) Reset() {
	*x = Groups{}
	mi := &file_daemon_started_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Groups) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Groups) ProtoMessage() {}

func (x *Groups) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Groups.ProtoReflect.Descriptor instead.
func (*Groups) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{6}
}

func (x *Groups) GetGroup() []*Group {
	if x != nil {
		return x.Group
	}
	return nil
}

type Group struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Tag           string                 `protobuf:"bytes,1,opt,name=tag,proto3" json:"tag,omitempty"`
	Type          string                 `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	Selectable    bool                   `protobuf:"varint,3,opt,name=selectable,proto3" json:"selectable,omitempty"`
	Selected      string                 `protobuf:"bytes,4,opt,name=selected,proto3" json:"selected,omitempty"`
	IsExpand      bool                   `protobuf:"varint,5,opt,name=isExpand,proto3" json:"isExpand,omitempty"`
	Items         []*GroupItem           `protobuf:"bytes,6,rep,name=items,proto3" json:"items,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Group) Reset() {
	*x = Group{}
	mi := &file_daemon_started_service_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Group) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Group) ProtoMessage() {}

func (x *Group) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Group.ProtoReflect.Descriptor instead.
func (*Group) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{7}
}

func (x *Group) GetTag() string {
	if x != nil {
		return x.Tag
	}
	return ""
}

func (x *Group) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Group) GetSelectable() bool {
	if x != nil {
		return x.Selectable
	}
	return false
}

func (x *Group) GetSelected() string {
	if x != nil {
		return x.Selected
	}
	return ""
}

func (x *Group) GetIsExpand() bool {
	if x != nil {
		return x.IsExpand
	}
	return false
}

func (x *Group) GetItems() []*GroupItem {
	if x != nil {
		return x.Items
	}
	return nil
}

type GroupItem struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Tag           string                 `protobuf:"bytes,1,opt,name=tag,proto3" json:"tag,omitempty"`
	Type          string                 `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	UrlTestTime   int64                  `protobuf:"varint,3,opt,name=urlTestTime,proto3" json:"urlTestTime,omitempty"`
	UrlTestDelay  int32                  `protobuf:"varint,4,opt,name=urlTestDelay,proto3" json:"urlTestDelay,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GroupItem) Reset() {
	*x = GroupItem{}
	mi := &file_daemon_started_service_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GroupItem) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GroupItem) ProtoMessage() {}

func (x *GroupItem) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GroupItem.ProtoReflect.Descriptor instead.
func (*GroupItem) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{8}
}

func (x *GroupItem) GetTag() string {
	if x != nil {
		return x.Tag
	}
	return ""
}

func (x *GroupItem) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *GroupItem) GetUrlTestTime() int64 {
	if x != nil {
		return x.UrlTestTime
	}
	return 0
}

func (x *GroupItem) GetUrlTestDelay() int32 {
	if x != nil {
		return x.UrlTestDelay
	}
	return 0
}

type URLTestRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	OutboundTag   string                 `protobuf:"bytes,1,opt,name=outboundTag,proto3" json:"outboundTag,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *URLTestRequest) Reset() {
	*x = URLTestRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *URLTestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*URLTestRequest) ProtoMessage() {}

func (x *URLTestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use URLTestRequest.ProtoReflect.Descriptor instead.
func (*URLTestRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{9}
}

func (x *URLTestRequest) GetOutboundTag() string {
	if x != nil {
		return x.OutboundTag
	}
	return ""
}

type SelectOutboundRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	GroupTag      string                 `protobuf:"bytes,1,opt,name=groupTag,proto3" json:"groupTag,omitempty"`
	OutboundTag   string                 `protobuf:"bytes,2,opt,name=outboundTag,proto3" json:"outboundTag,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SelectOutboundRequest) Reset() {
	*x = SelectOutboundRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SelectOutboundRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SelectOutboundRequest) ProtoMessage() {}

func (x *SelectOutboundRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SelectOutboundRequest.ProtoReflect.Descriptor instead.
func (*SelectOutboundRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{10}
}

func (x *SelectOutboundRequest) GetGroupTag() string {
	if x != nil {
		return x.GroupTag
	}
	return ""
}

func (x *SelectOutboundRequest) GetOutboundTag() string {
	if x != nil {
		return x.OutboundTag
	}
	return ""
}

type SetGroupExpandRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	GroupTag      string                 `protobuf:"bytes,1,opt,name=groupTag,proto3" json:"groupTag,omitempty"`
	IsExpand      bool                   `protobuf:"varint,2,opt,name=isExpand,proto3" json:"isExpand,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SetGroupExpandRequest) Reset() {
	*x = SetGroupExpandRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SetGroupExpandRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetGroupExpandRequest) ProtoMessage() {}

func (x *SetGroupExpandRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetGroupExpandRequest.ProtoReflect.Descriptor instead.
func (*SetGroupExpandRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{11}
}

func (x *SetGroupExpandRequest) GetGroupTag() string {
	if x != nil {
		return x.GroupTag
	}
	return ""
}

func (x *SetGroupExpandRequest) GetIsExpand() bool {
	if x != nil {
		return x.IsExpand
	}
	return false
}

type ClashMode struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mode          string                 `protobuf:"bytes,3,opt,name=mode,proto3" json:"mode,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClashMode) Reset() {
	*x = ClashMode{}
	mi := &file_daemon_started_service_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClashMode) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClashMode) ProtoMessage() {}

func (x *ClashMode) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClashMode.ProtoReflect.Descriptor instead.
func (*ClashMode) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{12}
}

func (x *ClashMode) GetMode() string {
	if x != nil {
		return x.Mode
	}
	return ""
}

type ClashModeStatus struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ModeList      []string               `protobuf:"bytes,1,rep,name=modeList,proto3" json:"modeList,omitempty"`
	CurrentMode   string                 `protobuf:"bytes,2,opt,name=currentMode,proto3" json:"currentMode,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClashModeStatus) Reset() {
	*x = ClashModeStatus{}
	mi := &file_daemon_started_service_proto_msgTypes[13]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClashModeStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClashModeStatus) ProtoMessage() {}

func (x *ClashModeStatus) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[13]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClashModeStatus.ProtoReflect.Descriptor instead.
func (*ClashModeStatus) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{13}
}

func (x *ClashModeStatus) GetModeList() []string {
	if x != nil {
		return x.ModeList
	}
	return nil
}

func (x *ClashModeStatus) GetCurrentMode() string {
	if x != nil {
		return x.CurrentMode
	}
	return ""
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
	mi := &file_daemon_started_service_proto_msgTypes[14]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SystemProxyStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemProxyStatus) ProtoMessage() {}

func (x *SystemProxyStatus) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[14]
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
	return file_daemon_started_service_proto_rawDescGZIP(), []int{14}
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
	mi := &file_daemon_started_service_proto_msgTypes[15]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SetSystemProxyEnabledRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetSystemProxyEnabledRequest) ProtoMessage() {}

func (x *SetSystemProxyEnabledRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[15]
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
	return file_daemon_started_service_proto_rawDescGZIP(), []int{15}
}

func (x *SetSystemProxyEnabledRequest) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

type SubscribeConnectionsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Interval      int64                  `protobuf:"varint,1,opt,name=interval,proto3" json:"interval,omitempty"`
	Filter        ConnectionFilter       `protobuf:"varint,2,opt,name=filter,proto3,enum=daemon.ConnectionFilter" json:"filter,omitempty"`
	SortBy        ConnectionSortBy       `protobuf:"varint,3,opt,name=sortBy,proto3,enum=daemon.ConnectionSortBy" json:"sortBy,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SubscribeConnectionsRequest) Reset() {
	*x = SubscribeConnectionsRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[16]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeConnectionsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeConnectionsRequest) ProtoMessage() {}

func (x *SubscribeConnectionsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[16]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeConnectionsRequest.ProtoReflect.Descriptor instead.
func (*SubscribeConnectionsRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{16}
}

func (x *SubscribeConnectionsRequest) GetInterval() int64 {
	if x != nil {
		return x.Interval
	}
	return 0
}

func (x *SubscribeConnectionsRequest) GetFilter() ConnectionFilter {
	if x != nil {
		return x.Filter
	}
	return ConnectionFilter_ALL
}

func (x *SubscribeConnectionsRequest) GetSortBy() ConnectionSortBy {
	if x != nil {
		return x.SortBy
	}
	return ConnectionSortBy_DATE
}

type Connections struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Connections   []*Connection          `protobuf:"bytes,1,rep,name=connections,proto3" json:"connections,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Connections) Reset() {
	*x = Connections{}
	mi := &file_daemon_started_service_proto_msgTypes[17]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Connections) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Connections) ProtoMessage() {}

func (x *Connections) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[17]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Connections.ProtoReflect.Descriptor instead.
func (*Connections) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{17}
}

func (x *Connections) GetConnections() []*Connection {
	if x != nil {
		return x.Connections
	}
	return nil
}

type Connection struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Inbound       string                 `protobuf:"bytes,2,opt,name=inbound,proto3" json:"inbound,omitempty"`
	InboundType   string                 `protobuf:"bytes,3,opt,name=inboundType,proto3" json:"inboundType,omitempty"`
	IpVersion     int32                  `protobuf:"varint,4,opt,name=ipVersion,proto3" json:"ipVersion,omitempty"`
	Network       string                 `protobuf:"bytes,5,opt,name=network,proto3" json:"network,omitempty"`
	Source        string                 `protobuf:"bytes,6,opt,name=source,proto3" json:"source,omitempty"`
	Destination   string                 `protobuf:"bytes,7,opt,name=destination,proto3" json:"destination,omitempty"`
	Domain        string                 `protobuf:"bytes,8,opt,name=domain,proto3" json:"domain,omitempty"`
	Protocol      string                 `protobuf:"bytes,9,opt,name=protocol,proto3" json:"protocol,omitempty"`
	User          string                 `protobuf:"bytes,10,opt,name=user,proto3" json:"user,omitempty"`
	FromOutbound  string                 `protobuf:"bytes,11,opt,name=fromOutbound,proto3" json:"fromOutbound,omitempty"`
	CreatedAt     int64                  `protobuf:"varint,12,opt,name=createdAt,proto3" json:"createdAt,omitempty"`
	ClosedAt      int64                  `protobuf:"varint,13,opt,name=closedAt,proto3" json:"closedAt,omitempty"`
	Uplink        int64                  `protobuf:"varint,14,opt,name=uplink,proto3" json:"uplink,omitempty"`
	Downlink      int64                  `protobuf:"varint,15,opt,name=downlink,proto3" json:"downlink,omitempty"`
	UplinkTotal   int64                  `protobuf:"varint,16,opt,name=uplinkTotal,proto3" json:"uplinkTotal,omitempty"`
	DownlinkTotal int64                  `protobuf:"varint,17,opt,name=downlinkTotal,proto3" json:"downlinkTotal,omitempty"`
	Rule          string                 `protobuf:"bytes,18,opt,name=rule,proto3" json:"rule,omitempty"`
	Outbound      string                 `protobuf:"bytes,19,opt,name=outbound,proto3" json:"outbound,omitempty"`
	OutboundType  string                 `protobuf:"bytes,20,opt,name=outboundType,proto3" json:"outboundType,omitempty"`
	ChainList     []string               `protobuf:"bytes,21,rep,name=chainList,proto3" json:"chainList,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Connection) Reset() {
	*x = Connection{}
	mi := &file_daemon_started_service_proto_msgTypes[18]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Connection) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Connection) ProtoMessage() {}

func (x *Connection) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[18]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Connection.ProtoReflect.Descriptor instead.
func (*Connection) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{18}
}

func (x *Connection) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Connection) GetInbound() string {
	if x != nil {
		return x.Inbound
	}
	return ""
}

func (x *Connection) GetInboundType() string {
	if x != nil {
		return x.InboundType
	}
	return ""
}

func (x *Connection) GetIpVersion() int32 {
	if x != nil {
		return x.IpVersion
	}
	return 0
}

func (x *Connection) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *Connection) GetSource() string {
	if x != nil {
		return x.Source
	}
	return ""
}

func (x *Connection) GetDestination() string {
	if x != nil {
		return x.Destination
	}
	return ""
}

func (x *Connection) GetDomain() string {
	if x != nil {
		return x.Domain
	}
	return ""
}

func (x *Connection) GetProtocol() string {
	if x != nil {
		return x.Protocol
	}
	return ""
}

func (x *Connection) GetUser() string {
	if x != nil {
		return x.User
	}
	return ""
}

func (x *Connection) GetFromOutbound() string {
	if x != nil {
		return x.FromOutbound
	}
	return ""
}

func (x *Connection) GetCreatedAt() int64 {
	if x != nil {
		return x.CreatedAt
	}
	return 0
}

func (x *Connection) GetClosedAt() int64 {
	if x != nil {
		return x.ClosedAt
	}
	return 0
}

func (x *Connection) GetUplink() int64 {
	if x != nil {
		return x.Uplink
	}
	return 0
}

func (x *Connection) GetDownlink() int64 {
	if x != nil {
		return x.Downlink
	}
	return 0
}

func (x *Connection) GetUplinkTotal() int64 {
	if x != nil {
		return x.UplinkTotal
	}
	return 0
}

func (x *Connection) GetDownlinkTotal() int64 {
	if x != nil {
		return x.DownlinkTotal
	}
	return 0
}

func (x *Connection) GetRule() string {
	if x != nil {
		return x.Rule
	}
	return ""
}

func (x *Connection) GetOutbound() string {
	if x != nil {
		return x.Outbound
	}
	return ""
}

func (x *Connection) GetOutboundType() string {
	if x != nil {
		return x.OutboundType
	}
	return ""
}

func (x *Connection) GetChainList() []string {
	if x != nil {
		return x.ChainList
	}
	return nil
}

type CloseConnectionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CloseConnectionRequest) Reset() {
	*x = CloseConnectionRequest{}
	mi := &file_daemon_started_service_proto_msgTypes[19]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CloseConnectionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloseConnectionRequest) ProtoMessage() {}

func (x *CloseConnectionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[19]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloseConnectionRequest.ProtoReflect.Descriptor instead.
func (*CloseConnectionRequest) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{19}
}

func (x *CloseConnectionRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type DeprecatedWarnings struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Warnings      []*DeprecatedWarning   `protobuf:"bytes,1,rep,name=warnings,proto3" json:"warnings,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DeprecatedWarnings) Reset() {
	*x = DeprecatedWarnings{}
	mi := &file_daemon_started_service_proto_msgTypes[20]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DeprecatedWarnings) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeprecatedWarnings) ProtoMessage() {}

func (x *DeprecatedWarnings) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[20]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeprecatedWarnings.ProtoReflect.Descriptor instead.
func (*DeprecatedWarnings) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{20}
}

func (x *DeprecatedWarnings) GetWarnings() []*DeprecatedWarning {
	if x != nil {
		return x.Warnings
	}
	return nil
}

type DeprecatedWarning struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       string                 `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	Impending     bool                   `protobuf:"varint,2,opt,name=impending,proto3" json:"impending,omitempty"`
	MigrationLink string                 `protobuf:"bytes,3,opt,name=migrationLink,proto3" json:"migrationLink,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DeprecatedWarning) Reset() {
	*x = DeprecatedWarning{}
	mi := &file_daemon_started_service_proto_msgTypes[21]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DeprecatedWarning) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeprecatedWarning) ProtoMessage() {}

func (x *DeprecatedWarning) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[21]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeprecatedWarning.ProtoReflect.Descriptor instead.
func (*DeprecatedWarning) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{21}
}

func (x *DeprecatedWarning) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *DeprecatedWarning) GetImpending() bool {
	if x != nil {
		return x.Impending
	}
	return false
}

func (x *DeprecatedWarning) GetMigrationLink() string {
	if x != nil {
		return x.MigrationLink
	}
	return ""
}

type Log_Message struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Level         LogLevel               `protobuf:"varint,1,opt,name=level,proto3,enum=daemon.LogLevel" json:"level,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Log_Message) Reset() {
	*x = Log_Message{}
	mi := &file_daemon_started_service_proto_msgTypes[22]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Log_Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Log_Message) ProtoMessage() {}

func (x *Log_Message) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_started_service_proto_msgTypes[22]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Log_Message.ProtoReflect.Descriptor instead.
func (*Log_Message) Descriptor() ([]byte, []int) {
	return file_daemon_started_service_proto_rawDescGZIP(), []int{3, 0}
}

func (x *Log_Message) GetLevel() LogLevel {
	if x != nil {
		return x.Level
	}
	return LogLevel_PANIC
}

func (x *Log_Message) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_daemon_started_service_proto protoreflect.FileDescriptor

const file_daemon_started_service_proto_rawDesc = "" +
	"\n" +
	"\x1cdaemon/started_service.proto\x12\x06daemon\x1a\x1bgoogle/protobuf/empty.proto\x1a\x13daemon/helper.proto\"\xad\x01\n" +
	"\rServiceStatus\x122\n" +
	"\x06status\x18\x01 \x01(\x0e2\x1a.daemon.ServiceStatus.TypeR\x06status\x12\"\n" +
	"\ferrorMessage\x18\x02 \x01(\tR\ferrorMessage\"D\n" +
	"\x04Type\x12\b\n" +
	"\x04IDLE\x10\x00\x12\f\n" +
	"\bSTARTING\x10\x01\x12\v\n" +
	"\aSTARTED\x10\x02\x12\f\n" +
	"\bSTOPPING\x10\x03\x12\t\n" +
	"\x05FATAL\x10\x04\"D\n" +
	"\x14ReloadServiceRequest\x12,\n" +
	"\x11newProfileContent\x18\x01 \x01(\tR\x11newProfileContent\"4\n" +
	"\x16SubscribeStatusRequest\x12\x1a\n" +
	"\binterval\x18\x01 \x01(\x03R\binterval\"\x99\x01\n" +
	"\x03Log\x12/\n" +
	"\bmessages\x18\x01 \x03(\v2\x13.daemon.Log.MessageR\bmessages\x12\x14\n" +
	"\x05reset\x18\x02 \x01(\bR\x05reset\x1aK\n" +
	"\aMessage\x12&\n" +
	"\x05level\x18\x01 \x01(\x0e2\x10.daemon.LogLevelR\x05level\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage\"9\n" +
	"\x0fDefaultLogLevel\x12&\n" +
	"\x05level\x18\x01 \x01(\x0e2\x10.daemon.LogLevelR\x05level\"\xb6\x02\n" +
	"\x06Status\x12\x16\n" +
	"\x06memory\x18\x01 \x01(\x04R\x06memory\x12\x1e\n" +
	"\n" +
	"goroutines\x18\x02 \x01(\x05R\n" +
	"goroutines\x12$\n" +
	"\rconnectionsIn\x18\x03 \x01(\x05R\rconnectionsIn\x12&\n" +
	"\x0econnectionsOut\x18\x04 \x01(\x05R\x0econnectionsOut\x12*\n" +
	"\x10trafficAvailable\x18\x05 \x01(\bR\x10trafficAvailable\x12\x16\n" +
	"\x06uplink\x18\x06 \x01(\x03R\x06uplink\x12\x1a\n" +
	"\bdownlink\x18\a \x01(\x03R\bdownlink\x12 \n" +
	"\vuplinkTotal\x18\b \x01(\x03R\vuplinkTotal\x12$\n" +
	"\rdownlinkTotal\x18\t \x01(\x03R\rdownlinkTotal\"-\n" +
	"\x06Groups\x12#\n" +
	"\x05group\x18\x01 \x03(\v2\r.daemon.GroupR\x05group\"\xae\x01\n" +
	"\x05Group\x12\x10\n" +
	"\x03tag\x18\x01 \x01(\tR\x03tag\x12\x12\n" +
	"\x04type\x18\x02 \x01(\tR\x04type\x12\x1e\n" +
	"\n" +
	"selectable\x18\x03 \x01(\bR\n" +
	"selectable\x12\x1a\n" +
	"\bselected\x18\x04 \x01(\tR\bselected\x12\x1a\n" +
	"\bisExpand\x18\x05 \x01(\bR\bisExpand\x12'\n" +
	"\x05items\x18\x06 \x03(\v2\x11.daemon.GroupItemR\x05items\"w\n" +
	"\tGroupItem\x12\x10\n" +
	"\x03tag\x18\x01 \x01(\tR\x03tag\x12\x12\n" +
	"\x04type\x18\x02 \x01(\tR\x04type\x12 \n" +
	"\vurlTestTime\x18\x03 \x01(\x03R\vurlTestTime\x12\"\n" +
	"\furlTestDelay\x18\x04 \x01(\x05R\furlTestDelay\"2\n" +
	"\x0eURLTestRequest\x12 \n" +
	"\voutboundTag\x18\x01 \x01(\tR\voutboundTag\"U\n" +
	"\x15SelectOutboundRequest\x12\x1a\n" +
	"\bgroupTag\x18\x01 \x01(\tR\bgroupTag\x12 \n" +
	"\voutboundTag\x18\x02 \x01(\tR\voutboundTag\"O\n" +
	"\x15SetGroupExpandRequest\x12\x1a\n" +
	"\bgroupTag\x18\x01 \x01(\tR\bgroupTag\x12\x1a\n" +
	"\bisExpand\x18\x02 \x01(\bR\bisExpand\"\x1f\n" +
	"\tClashMode\x12\x12\n" +
	"\x04mode\x18\x03 \x01(\tR\x04mode\"O\n" +
	"\x0fClashModeStatus\x12\x1a\n" +
	"\bmodeList\x18\x01 \x03(\tR\bmodeList\x12 \n" +
	"\vcurrentMode\x18\x02 \x01(\tR\vcurrentMode\"K\n" +
	"\x11SystemProxyStatus\x12\x1c\n" +
	"\tavailable\x18\x01 \x01(\bR\tavailable\x12\x18\n" +
	"\aenabled\x18\x02 \x01(\bR\aenabled\"8\n" +
	"\x1cSetSystemProxyEnabledRequest\x12\x18\n" +
	"\aenabled\x18\x01 \x01(\bR\aenabled\"\x9d\x01\n" +
	"\x1bSubscribeConnectionsRequest\x12\x1a\n" +
	"\binterval\x18\x01 \x01(\x03R\binterval\x120\n" +
	"\x06filter\x18\x02 \x01(\x0e2\x18.daemon.ConnectionFilterR\x06filter\x120\n" +
	"\x06sortBy\x18\x03 \x01(\x0e2\x18.daemon.ConnectionSortByR\x06sortBy\"C\n" +
	"\vConnections\x124\n" +
	"\vconnections\x18\x01 \x03(\v2\x12.daemon.ConnectionR\vconnections\"\xde\x04\n" +
	"\n" +
	"Connection\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x18\n" +
	"\ainbound\x18\x02 \x01(\tR\ainbound\x12 \n" +
	"\vinboundType\x18\x03 \x01(\tR\vinboundType\x12\x1c\n" +
	"\tipVersion\x18\x04 \x01(\x05R\tipVersion\x12\x18\n" +
	"\anetwork\x18\x05 \x01(\tR\anetwork\x12\x16\n" +
	"\x06source\x18\x06 \x01(\tR\x06source\x12 \n" +
	"\vdestination\x18\a \x01(\tR\vdestination\x12\x16\n" +
	"\x06domain\x18\b \x01(\tR\x06domain\x12\x1a\n" +
	"\bprotocol\x18\t \x01(\tR\bprotocol\x12\x12\n" +
	"\x04user\x18\n" +
	" \x01(\tR\x04user\x12\"\n" +
	"\ffromOutbound\x18\v \x01(\tR\ffromOutbound\x12\x1c\n" +
	"\tcreatedAt\x18\f \x01(\x03R\tcreatedAt\x12\x1a\n" +
	"\bclosedAt\x18\r \x01(\x03R\bclosedAt\x12\x16\n" +
	"\x06uplink\x18\x0e \x01(\x03R\x06uplink\x12\x1a\n" +
	"\bdownlink\x18\x0f \x01(\x03R\bdownlink\x12 \n" +
	"\vuplinkTotal\x18\x10 \x01(\x03R\vuplinkTotal\x12$\n" +
	"\rdownlinkTotal\x18\x11 \x01(\x03R\rdownlinkTotal\x12\x12\n" +
	"\x04rule\x18\x12 \x01(\tR\x04rule\x12\x1a\n" +
	"\boutbound\x18\x13 \x01(\tR\boutbound\x12\"\n" +
	"\foutboundType\x18\x14 \x01(\tR\foutboundType\x12\x1c\n" +
	"\tchainList\x18\x15 \x03(\tR\tchainList\"(\n" +
	"\x16CloseConnectionRequest\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\"K\n" +
	"\x12DeprecatedWarnings\x125\n" +
	"\bwarnings\x18\x01 \x03(\v2\x19.daemon.DeprecatedWarningR\bwarnings\"q\n" +
	"\x11DeprecatedWarning\x12\x18\n" +
	"\amessage\x18\x01 \x01(\tR\amessage\x12\x1c\n" +
	"\timpending\x18\x02 \x01(\bR\timpending\x12$\n" +
	"\rmigrationLink\x18\x03 \x01(\tR\rmigrationLink*U\n" +
	"\bLogLevel\x12\t\n" +
	"\x05PANIC\x10\x00\x12\t\n" +
	"\x05FATAL\x10\x01\x12\t\n" +
	"\x05ERROR\x10\x02\x12\b\n" +
	"\x04WARN\x10\x03\x12\b\n" +
	"\x04INFO\x10\x04\x12\t\n" +
	"\x05DEBUG\x10\x05\x12\t\n" +
	"\x05TRACE\x10\x06*3\n" +
	"\x10ConnectionFilter\x12\a\n" +
	"\x03ALL\x10\x00\x12\n" +
	"\n" +
	"\x06ACTIVE\x10\x01\x12\n" +
	"\n" +
	"\x06CLOSED\x10\x02*<\n" +
	"\x10ConnectionSortBy\x12\b\n" +
	"\x04DATE\x10\x00\x12\v\n" +
	"\aTRAFFIC\x10\x01\x12\x11\n" +
	"\rTOTAL_TRAFFIC\x10\x022\xb7\f\n" +
	"\x0eStartedService\x12=\n" +
	"\vStopService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\x12?\n" +
	"\rReloadService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\x12K\n" +
	"\x16SubscribeServiceStatus\x12\x16.google.protobuf.Empty\x1a\x15.daemon.ServiceStatus\"\x000\x01\x127\n" +
	"\fSubscribeLog\x12\x16.google.protobuf.Empty\x1a\v.daemon.Log\"\x000\x01\x12G\n" +
	"\x12GetDefaultLogLevel\x12\x16.google.protobuf.Empty\x1a\x17.daemon.DefaultLogLevel\"\x00\x12=\n" +
	"\tClearLogs\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12E\n" +
	"\x0fSubscribeStatus\x12\x1e.daemon.SubscribeStatusRequest\x1a\x0e.daemon.Status\"\x000\x01\x12=\n" +
	"\x0fSubscribeGroups\x12\x16.google.protobuf.Empty\x1a\x0e.daemon.Groups\"\x000\x01\x12G\n" +
	"\x12GetClashModeStatus\x12\x16.google.protobuf.Empty\x1a\x17.daemon.ClashModeStatus\"\x00\x12C\n" +
	"\x12SubscribeClashMode\x12\x16.google.protobuf.Empty\x1a\x11.daemon.ClashMode\"\x000\x01\x12;\n" +
	"\fSetClashMode\x12\x11.daemon.ClashMode\x1a\x16.google.protobuf.Empty\"\x00\x12;\n" +
	"\aURLTest\x12\x16.daemon.URLTestRequest\x1a\x16.google.protobuf.Empty\"\x00\x12I\n" +
	"\x0eSelectOutbound\x12\x1d.daemon.SelectOutboundRequest\x1a\x16.google.protobuf.Empty\"\x00\x12I\n" +
	"\x0eSetGroupExpand\x12\x1d.daemon.SetGroupExpandRequest\x1a\x16.google.protobuf.Empty\"\x00\x12K\n" +
	"\x14GetSystemProxyStatus\x12\x16.google.protobuf.Empty\x1a\x19.daemon.SystemProxyStatus\"\x00\x12W\n" +
	"\x15SetSystemProxyEnabled\x12$.daemon.SetSystemProxyEnabledRequest\x1a\x16.google.protobuf.Empty\"\x00\x12T\n" +
	"\x14SubscribeConnections\x12#.daemon.SubscribeConnectionsRequest\x1a\x13.daemon.Connections\"\x000\x01\x12K\n" +
	"\x0fCloseConnection\x12\x1e.daemon.CloseConnectionRequest\x1a\x16.google.protobuf.Empty\"\x00\x12G\n" +
	"\x13CloseAllConnections\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12M\n" +
	"\x15GetDeprecatedWarnings\x12\x16.google.protobuf.Empty\x1a\x1a.daemon.DeprecatedWarnings\"\x00\x12J\n" +
	"\x15SubscribeHelperEvents\x12\x16.google.protobuf.Empty\x1a\x15.daemon.HelperRequest\"\x000\x01\x12F\n" +
	"\x12SendHelperResponse\x12\x16.daemon.HelperResponse\x1a\x16.google.protobuf.Empty\"\x00B%Z#github.com/sagernet/sing-box/daemonb\x06proto3"

var (
	file_daemon_started_service_proto_rawDescOnce sync.Once
	file_daemon_started_service_proto_rawDescData []byte
)

func file_daemon_started_service_proto_rawDescGZIP() []byte {
	file_daemon_started_service_proto_rawDescOnce.Do(func() {
		file_daemon_started_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_daemon_started_service_proto_rawDesc), len(file_daemon_started_service_proto_rawDesc)))
	})
	return file_daemon_started_service_proto_rawDescData
}

var (
	file_daemon_started_service_proto_enumTypes = make([]protoimpl.EnumInfo, 4)
	file_daemon_started_service_proto_msgTypes  = make([]protoimpl.MessageInfo, 23)
	file_daemon_started_service_proto_goTypes   = []any{
		(LogLevel)(0),                        // 0: daemon.LogLevel
		(ConnectionFilter)(0),                // 1: daemon.ConnectionFilter
		(ConnectionSortBy)(0),                // 2: daemon.ConnectionSortBy
		(ServiceStatus_Type)(0),              // 3: daemon.ServiceStatus.Type
		(*ServiceStatus)(nil),                // 4: daemon.ServiceStatus
		(*ReloadServiceRequest)(nil),         // 5: daemon.ReloadServiceRequest
		(*SubscribeStatusRequest)(nil),       // 6: daemon.SubscribeStatusRequest
		(*Log)(nil),                          // 7: daemon.Log
		(*DefaultLogLevel)(nil),              // 8: daemon.DefaultLogLevel
		(*Status)(nil),                       // 9: daemon.Status
		(*Groups)(nil),                       // 10: daemon.Groups
		(*Group)(nil),                        // 11: daemon.Group
		(*GroupItem)(nil),                    // 12: daemon.GroupItem
		(*URLTestRequest)(nil),               // 13: daemon.URLTestRequest
		(*SelectOutboundRequest)(nil),        // 14: daemon.SelectOutboundRequest
		(*SetGroupExpandRequest)(nil),        // 15: daemon.SetGroupExpandRequest
		(*ClashMode)(nil),                    // 16: daemon.ClashMode
		(*ClashModeStatus)(nil),              // 17: daemon.ClashModeStatus
		(*SystemProxyStatus)(nil),            // 18: daemon.SystemProxyStatus
		(*SetSystemProxyEnabledRequest)(nil), // 19: daemon.SetSystemProxyEnabledRequest
		(*SubscribeConnectionsRequest)(nil),  // 20: daemon.SubscribeConnectionsRequest
		(*Connections)(nil),                  // 21: daemon.Connections
		(*Connection)(nil),                   // 22: daemon.Connection
		(*CloseConnectionRequest)(nil),       // 23: daemon.CloseConnectionRequest
		(*DeprecatedWarnings)(nil),           // 24: daemon.DeprecatedWarnings
		(*DeprecatedWarning)(nil),            // 25: daemon.DeprecatedWarning
		(*Log_Message)(nil),                  // 26: daemon.Log.Message
		(*emptypb.Empty)(nil),                // 27: google.protobuf.Empty
		(*HelperResponse)(nil),               // 28: daemon.HelperResponse
		(*HelperRequest)(nil),                // 29: daemon.HelperRequest
	}
)

var file_daemon_started_service_proto_depIdxs = []int32{
	3,  // 0: daemon.ServiceStatus.status:type_name -> daemon.ServiceStatus.Type
	26, // 1: daemon.Log.messages:type_name -> daemon.Log.Message
	0,  // 2: daemon.DefaultLogLevel.level:type_name -> daemon.LogLevel
	11, // 3: daemon.Groups.group:type_name -> daemon.Group
	12, // 4: daemon.Group.items:type_name -> daemon.GroupItem
	1,  // 5: daemon.SubscribeConnectionsRequest.filter:type_name -> daemon.ConnectionFilter
	2,  // 6: daemon.SubscribeConnectionsRequest.sortBy:type_name -> daemon.ConnectionSortBy
	22, // 7: daemon.Connections.connections:type_name -> daemon.Connection
	25, // 8: daemon.DeprecatedWarnings.warnings:type_name -> daemon.DeprecatedWarning
	0,  // 9: daemon.Log.Message.level:type_name -> daemon.LogLevel
	27, // 10: daemon.StartedService.StopService:input_type -> google.protobuf.Empty
	27, // 11: daemon.StartedService.ReloadService:input_type -> google.protobuf.Empty
	27, // 12: daemon.StartedService.SubscribeServiceStatus:input_type -> google.protobuf.Empty
	27, // 13: daemon.StartedService.SubscribeLog:input_type -> google.protobuf.Empty
	27, // 14: daemon.StartedService.GetDefaultLogLevel:input_type -> google.protobuf.Empty
	27, // 15: daemon.StartedService.ClearLogs:input_type -> google.protobuf.Empty
	6,  // 16: daemon.StartedService.SubscribeStatus:input_type -> daemon.SubscribeStatusRequest
	27, // 17: daemon.StartedService.SubscribeGroups:input_type -> google.protobuf.Empty
	27, // 18: daemon.StartedService.GetClashModeStatus:input_type -> google.protobuf.Empty
	27, // 19: daemon.StartedService.SubscribeClashMode:input_type -> google.protobuf.Empty
	16, // 20: daemon.StartedService.SetClashMode:input_type -> daemon.ClashMode
	13, // 21: daemon.StartedService.URLTest:input_type -> daemon.URLTestRequest
	14, // 22: daemon.StartedService.SelectOutbound:input_type -> daemon.SelectOutboundRequest
	15, // 23: daemon.StartedService.SetGroupExpand:input_type -> daemon.SetGroupExpandRequest
	27, // 24: daemon.StartedService.GetSystemProxyStatus:input_type -> google.protobuf.Empty
	19, // 25: daemon.StartedService.SetSystemProxyEnabled:input_type -> daemon.SetSystemProxyEnabledRequest
	20, // 26: daemon.StartedService.SubscribeConnections:input_type -> daemon.SubscribeConnectionsRequest
	23, // 27: daemon.StartedService.CloseConnection:input_type -> daemon.CloseConnectionRequest
	27, // 28: daemon.StartedService.CloseAllConnections:input_type -> google.protobuf.Empty
	27, // 29: daemon.StartedService.GetDeprecatedWarnings:input_type -> google.protobuf.Empty
	27, // 30: daemon.StartedService.SubscribeHelperEvents:input_type -> google.protobuf.Empty
	28, // 31: daemon.StartedService.SendHelperResponse:input_type -> daemon.HelperResponse
	27, // 32: daemon.StartedService.StopService:output_type -> google.protobuf.Empty
	27, // 33: daemon.StartedService.ReloadService:output_type -> google.protobuf.Empty
	4,  // 34: daemon.StartedService.SubscribeServiceStatus:output_type -> daemon.ServiceStatus
	7,  // 35: daemon.StartedService.SubscribeLog:output_type -> daemon.Log
	8,  // 36: daemon.StartedService.GetDefaultLogLevel:output_type -> daemon.DefaultLogLevel
	27, // 37: daemon.StartedService.ClearLogs:output_type -> google.protobuf.Empty
	9,  // 38: daemon.StartedService.SubscribeStatus:output_type -> daemon.Status
	10, // 39: daemon.StartedService.SubscribeGroups:output_type -> daemon.Groups
	17, // 40: daemon.StartedService.GetClashModeStatus:output_type -> daemon.ClashModeStatus
	16, // 41: daemon.StartedService.SubscribeClashMode:output_type -> daemon.ClashMode
	27, // 42: daemon.StartedService.SetClashMode:output_type -> google.protobuf.Empty
	27, // 43: daemon.StartedService.URLTest:output_type -> google.protobuf.Empty
	27, // 44: daemon.StartedService.SelectOutbound:output_type -> google.protobuf.Empty
	27, // 45: daemon.StartedService.SetGroupExpand:output_type -> google.protobuf.Empty
	18, // 46: daemon.StartedService.GetSystemProxyStatus:output_type -> daemon.SystemProxyStatus
	27, // 47: daemon.StartedService.SetSystemProxyEnabled:output_type -> google.protobuf.Empty
	21, // 48: daemon.StartedService.SubscribeConnections:output_type -> daemon.Connections
	27, // 49: daemon.StartedService.CloseConnection:output_type -> google.protobuf.Empty
	27, // 50: daemon.StartedService.CloseAllConnections:output_type -> google.protobuf.Empty
	24, // 51: daemon.StartedService.GetDeprecatedWarnings:output_type -> daemon.DeprecatedWarnings
	29, // 52: daemon.StartedService.SubscribeHelperEvents:output_type -> daemon.HelperRequest
	27, // 53: daemon.StartedService.SendHelperResponse:output_type -> google.protobuf.Empty
	32, // [32:54] is the sub-list for method output_type
	10, // [10:32] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_daemon_started_service_proto_init() }
func file_daemon_started_service_proto_init() {
	if File_daemon_started_service_proto != nil {
		return
	}
	file_daemon_helper_proto_init()
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_daemon_started_service_proto_rawDesc), len(file_daemon_started_service_proto_rawDesc)),
			NumEnums:      4,
			NumMessages:   23,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_daemon_started_service_proto_goTypes,
		DependencyIndexes: file_daemon_started_service_proto_depIdxs,
		EnumInfos:         file_daemon_started_service_proto_enumTypes,
		MessageInfos:      file_daemon_started_service_proto_msgTypes,
	}.Build()
	File_daemon_started_service_proto = out.File
	file_daemon_started_service_proto_goTypes = nil
	file_daemon_started_service_proto_depIdxs = nil
}
