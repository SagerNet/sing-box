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

type SubscribeHelperRequestRequest struct {
	state                             protoimpl.MessageState `protogen:"open.v1"`
	AcceptGetWIFIStateRequests        bool                   `protobuf:"varint,1,opt,name=acceptGetWIFIStateRequests,proto3" json:"acceptGetWIFIStateRequests,omitempty"`
	AcceptFindConnectionOwnerRequests bool                   `protobuf:"varint,2,opt,name=acceptFindConnectionOwnerRequests,proto3" json:"acceptFindConnectionOwnerRequests,omitempty"`
	AcceptSendNotificationRequests    bool                   `protobuf:"varint,3,opt,name=acceptSendNotificationRequests,proto3" json:"acceptSendNotificationRequests,omitempty"`
	unknownFields                     protoimpl.UnknownFields
	sizeCache                         protoimpl.SizeCache
}

func (x *SubscribeHelperRequestRequest) Reset() {
	*x = SubscribeHelperRequestRequest{}
	mi := &file_daemon_helper_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeHelperRequestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeHelperRequestRequest) ProtoMessage() {}

func (x *SubscribeHelperRequestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeHelperRequestRequest.ProtoReflect.Descriptor instead.
func (*SubscribeHelperRequestRequest) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{0}
}

func (x *SubscribeHelperRequestRequest) GetAcceptGetWIFIStateRequests() bool {
	if x != nil {
		return x.AcceptGetWIFIStateRequests
	}
	return false
}

func (x *SubscribeHelperRequestRequest) GetAcceptFindConnectionOwnerRequests() bool {
	if x != nil {
		return x.AcceptFindConnectionOwnerRequests
	}
	return false
}

func (x *SubscribeHelperRequestRequest) GetAcceptSendNotificationRequests() bool {
	if x != nil {
		return x.AcceptSendNotificationRequests
	}
	return false
}

type HelperRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	Id    int64                  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Types that are valid to be assigned to Request:
	//
	//	*HelperRequest_GetWIFIState
	//	*HelperRequest_FindConnectionOwner
	//	*HelperRequest_SendNotification
	Request       isHelperRequest_Request `protobuf_oneof:"request"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HelperRequest) Reset() {
	*x = HelperRequest{}
	mi := &file_daemon_helper_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HelperRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HelperRequest) ProtoMessage() {}

func (x *HelperRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HelperRequest.ProtoReflect.Descriptor instead.
func (*HelperRequest) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{1}
}

func (x *HelperRequest) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *HelperRequest) GetRequest() isHelperRequest_Request {
	if x != nil {
		return x.Request
	}
	return nil
}

func (x *HelperRequest) GetGetWIFIState() *emptypb.Empty {
	if x != nil {
		if x, ok := x.Request.(*HelperRequest_GetWIFIState); ok {
			return x.GetWIFIState
		}
	}
	return nil
}

func (x *HelperRequest) GetFindConnectionOwner() *FindConnectionOwnerRequest {
	if x != nil {
		if x, ok := x.Request.(*HelperRequest_FindConnectionOwner); ok {
			return x.FindConnectionOwner
		}
	}
	return nil
}

func (x *HelperRequest) GetSendNotification() *Notification {
	if x != nil {
		if x, ok := x.Request.(*HelperRequest_SendNotification); ok {
			return x.SendNotification
		}
	}
	return nil
}

type isHelperRequest_Request interface {
	isHelperRequest_Request()
}

type HelperRequest_GetWIFIState struct {
	GetWIFIState *emptypb.Empty `protobuf:"bytes,2,opt,name=getWIFIState,proto3,oneof"`
}

type HelperRequest_FindConnectionOwner struct {
	FindConnectionOwner *FindConnectionOwnerRequest `protobuf:"bytes,3,opt,name=findConnectionOwner,proto3,oneof"`
}

type HelperRequest_SendNotification struct {
	SendNotification *Notification `protobuf:"bytes,4,opt,name=sendNotification,proto3,oneof"`
}

func (*HelperRequest_GetWIFIState) isHelperRequest_Request() {}

func (*HelperRequest_FindConnectionOwner) isHelperRequest_Request() {}

func (*HelperRequest_SendNotification) isHelperRequest_Request() {}

type FindConnectionOwnerRequest struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	IpProtocol         int32                  `protobuf:"varint,1,opt,name=ipProtocol,proto3" json:"ipProtocol,omitempty"`
	SourceAddress      string                 `protobuf:"bytes,2,opt,name=sourceAddress,proto3" json:"sourceAddress,omitempty"`
	SourcePort         int32                  `protobuf:"varint,3,opt,name=sourcePort,proto3" json:"sourcePort,omitempty"`
	DestinationAddress string                 `protobuf:"bytes,4,opt,name=destinationAddress,proto3" json:"destinationAddress,omitempty"`
	DestinationPort    int32                  `protobuf:"varint,5,opt,name=destinationPort,proto3" json:"destinationPort,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *FindConnectionOwnerRequest) Reset() {
	*x = FindConnectionOwnerRequest{}
	mi := &file_daemon_helper_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FindConnectionOwnerRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FindConnectionOwnerRequest) ProtoMessage() {}

func (x *FindConnectionOwnerRequest) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FindConnectionOwnerRequest.ProtoReflect.Descriptor instead.
func (*FindConnectionOwnerRequest) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{2}
}

func (x *FindConnectionOwnerRequest) GetIpProtocol() int32 {
	if x != nil {
		return x.IpProtocol
	}
	return 0
}

func (x *FindConnectionOwnerRequest) GetSourceAddress() string {
	if x != nil {
		return x.SourceAddress
	}
	return ""
}

func (x *FindConnectionOwnerRequest) GetSourcePort() int32 {
	if x != nil {
		return x.SourcePort
	}
	return 0
}

func (x *FindConnectionOwnerRequest) GetDestinationAddress() string {
	if x != nil {
		return x.DestinationAddress
	}
	return ""
}

func (x *FindConnectionOwnerRequest) GetDestinationPort() int32 {
	if x != nil {
		return x.DestinationPort
	}
	return 0
}

type Notification struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Identifier    string                 `protobuf:"bytes,1,opt,name=identifier,proto3" json:"identifier,omitempty"`
	TypeName      string                 `protobuf:"bytes,2,opt,name=typeName,proto3" json:"typeName,omitempty"`
	TypeId        int32                  `protobuf:"varint,3,opt,name=typeId,proto3" json:"typeId,omitempty"`
	Title         string                 `protobuf:"bytes,4,opt,name=title,proto3" json:"title,omitempty"`
	Subtitle      string                 `protobuf:"bytes,5,opt,name=subtitle,proto3" json:"subtitle,omitempty"`
	Body          string                 `protobuf:"bytes,6,opt,name=body,proto3" json:"body,omitempty"`
	OpenURL       string                 `protobuf:"bytes,7,opt,name=openURL,proto3" json:"openURL,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Notification) Reset() {
	*x = Notification{}
	mi := &file_daemon_helper_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Notification) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Notification) ProtoMessage() {}

func (x *Notification) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Notification.ProtoReflect.Descriptor instead.
func (*Notification) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{3}
}

func (x *Notification) GetIdentifier() string {
	if x != nil {
		return x.Identifier
	}
	return ""
}

func (x *Notification) GetTypeName() string {
	if x != nil {
		return x.TypeName
	}
	return ""
}

func (x *Notification) GetTypeId() int32 {
	if x != nil {
		return x.TypeId
	}
	return 0
}

func (x *Notification) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *Notification) GetSubtitle() string {
	if x != nil {
		return x.Subtitle
	}
	return ""
}

func (x *Notification) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

func (x *Notification) GetOpenURL() string {
	if x != nil {
		return x.OpenURL
	}
	return ""
}

type HelperResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	Id    int64                  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Types that are valid to be assigned to Response:
	//
	//	*HelperResponse_WifiState
	//	*HelperResponse_Error
	//	*HelperResponse_ConnectionOwner
	Response      isHelperResponse_Response `protobuf_oneof:"response"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HelperResponse) Reset() {
	*x = HelperResponse{}
	mi := &file_daemon_helper_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HelperResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HelperResponse) ProtoMessage() {}

func (x *HelperResponse) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HelperResponse.ProtoReflect.Descriptor instead.
func (*HelperResponse) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{4}
}

func (x *HelperResponse) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *HelperResponse) GetResponse() isHelperResponse_Response {
	if x != nil {
		return x.Response
	}
	return nil
}

func (x *HelperResponse) GetWifiState() *WIFIState {
	if x != nil {
		if x, ok := x.Response.(*HelperResponse_WifiState); ok {
			return x.WifiState
		}
	}
	return nil
}

func (x *HelperResponse) GetError() string {
	if x != nil {
		if x, ok := x.Response.(*HelperResponse_Error); ok {
			return x.Error
		}
	}
	return ""
}

func (x *HelperResponse) GetConnectionOwner() *ConnectionOwner {
	if x != nil {
		if x, ok := x.Response.(*HelperResponse_ConnectionOwner); ok {
			return x.ConnectionOwner
		}
	}
	return nil
}

type isHelperResponse_Response interface {
	isHelperResponse_Response()
}

type HelperResponse_WifiState struct {
	WifiState *WIFIState `protobuf:"bytes,2,opt,name=wifiState,proto3,oneof"`
}

type HelperResponse_Error struct {
	Error string `protobuf:"bytes,3,opt,name=error,proto3,oneof"`
}

type HelperResponse_ConnectionOwner struct {
	ConnectionOwner *ConnectionOwner `protobuf:"bytes,4,opt,name=connectionOwner,proto3,oneof"`
}

func (*HelperResponse_WifiState) isHelperResponse_Response() {}

func (*HelperResponse_Error) isHelperResponse_Response() {}

func (*HelperResponse_ConnectionOwner) isHelperResponse_Response() {}

type ConnectionOwner struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	UserId             int32                  `protobuf:"varint,1,opt,name=userId,proto3" json:"userId,omitempty"`
	UserName           string                 `protobuf:"bytes,2,opt,name=userName,proto3" json:"userName,omitempty"`
	ProcessPath        string                 `protobuf:"bytes,3,opt,name=processPath,proto3" json:"processPath,omitempty"`
	AndroidPackageName string                 `protobuf:"bytes,4,opt,name=androidPackageName,proto3" json:"androidPackageName,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *ConnectionOwner) Reset() {
	*x = ConnectionOwner{}
	mi := &file_daemon_helper_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConnectionOwner) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConnectionOwner) ProtoMessage() {}

func (x *ConnectionOwner) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConnectionOwner.ProtoReflect.Descriptor instead.
func (*ConnectionOwner) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{5}
}

func (x *ConnectionOwner) GetUserId() int32 {
	if x != nil {
		return x.UserId
	}
	return 0
}

func (x *ConnectionOwner) GetUserName() string {
	if x != nil {
		return x.UserName
	}
	return ""
}

func (x *ConnectionOwner) GetProcessPath() string {
	if x != nil {
		return x.ProcessPath
	}
	return ""
}

func (x *ConnectionOwner) GetAndroidPackageName() string {
	if x != nil {
		return x.AndroidPackageName
	}
	return ""
}

type WIFIState struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Ssid          string                 `protobuf:"bytes,1,opt,name=ssid,proto3" json:"ssid,omitempty"`
	Bssid         string                 `protobuf:"bytes,2,opt,name=bssid,proto3" json:"bssid,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WIFIState) Reset() {
	*x = WIFIState{}
	mi := &file_daemon_helper_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WIFIState) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WIFIState) ProtoMessage() {}

func (x *WIFIState) ProtoReflect() protoreflect.Message {
	mi := &file_daemon_helper_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WIFIState.ProtoReflect.Descriptor instead.
func (*WIFIState) Descriptor() ([]byte, []int) {
	return file_daemon_helper_proto_rawDescGZIP(), []int{6}
}

func (x *WIFIState) GetSsid() string {
	if x != nil {
		return x.Ssid
	}
	return ""
}

func (x *WIFIState) GetBssid() string {
	if x != nil {
		return x.Bssid
	}
	return ""
}

var File_daemon_helper_proto protoreflect.FileDescriptor

const file_daemon_helper_proto_rawDesc = "" +
	"\n" +
	"\x13daemon/helper.proto\x12\x06daemon\x1a\x1bgoogle/protobuf/empty.proto\"\xf5\x01\n" +
	"\x1dSubscribeHelperRequestRequest\x12>\n" +
	"\x1aacceptGetWIFIStateRequests\x18\x01 \x01(\bR\x1aacceptGetWIFIStateRequests\x12L\n" +
	"!acceptFindConnectionOwnerRequests\x18\x02 \x01(\bR!acceptFindConnectionOwnerRequests\x12F\n" +
	"\x1eacceptSendNotificationRequests\x18\x03 \x01(\bR\x1eacceptSendNotificationRequests\"\x84\x02\n" +
	"\rHelperRequest\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\x03R\x02id\x12<\n" +
	"\fgetWIFIState\x18\x02 \x01(\v2\x16.google.protobuf.EmptyH\x00R\fgetWIFIState\x12V\n" +
	"\x13findConnectionOwner\x18\x03 \x01(\v2\".daemon.FindConnectionOwnerRequestH\x00R\x13findConnectionOwner\x12B\n" +
	"\x10sendNotification\x18\x04 \x01(\v2\x14.daemon.NotificationH\x00R\x10sendNotificationB\t\n" +
	"\arequest\"\xdc\x01\n" +
	"\x1aFindConnectionOwnerRequest\x12\x1e\n" +
	"\n" +
	"ipProtocol\x18\x01 \x01(\x05R\n" +
	"ipProtocol\x12$\n" +
	"\rsourceAddress\x18\x02 \x01(\tR\rsourceAddress\x12\x1e\n" +
	"\n" +
	"sourcePort\x18\x03 \x01(\x05R\n" +
	"sourcePort\x12.\n" +
	"\x12destinationAddress\x18\x04 \x01(\tR\x12destinationAddress\x12(\n" +
	"\x0fdestinationPort\x18\x05 \x01(\x05R\x0fdestinationPort\"\xc2\x01\n" +
	"\fNotification\x12\x1e\n" +
	"\n" +
	"identifier\x18\x01 \x01(\tR\n" +
	"identifier\x12\x1a\n" +
	"\btypeName\x18\x02 \x01(\tR\btypeName\x12\x16\n" +
	"\x06typeId\x18\x03 \x01(\x05R\x06typeId\x12\x14\n" +
	"\x05title\x18\x04 \x01(\tR\x05title\x12\x1a\n" +
	"\bsubtitle\x18\x05 \x01(\tR\bsubtitle\x12\x12\n" +
	"\x04body\x18\x06 \x01(\tR\x04body\x12\x18\n" +
	"\aopenURL\x18\a \x01(\tR\aopenURL\"\xbc\x01\n" +
	"\x0eHelperResponse\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\x03R\x02id\x121\n" +
	"\twifiState\x18\x02 \x01(\v2\x11.daemon.WIFIStateH\x00R\twifiState\x12\x16\n" +
	"\x05error\x18\x03 \x01(\tH\x00R\x05error\x12C\n" +
	"\x0fconnectionOwner\x18\x04 \x01(\v2\x17.daemon.ConnectionOwnerH\x00R\x0fconnectionOwnerB\n" +
	"\n" +
	"\bresponse\"\x97\x01\n" +
	"\x0fConnectionOwner\x12\x16\n" +
	"\x06userId\x18\x01 \x01(\x05R\x06userId\x12\x1a\n" +
	"\buserName\x18\x02 \x01(\tR\buserName\x12 \n" +
	"\vprocessPath\x18\x03 \x01(\tR\vprocessPath\x12.\n" +
	"\x12androidPackageName\x18\x04 \x01(\tR\x12androidPackageName\"5\n" +
	"\tWIFIState\x12\x12\n" +
	"\x04ssid\x18\x01 \x01(\tR\x04ssid\x12\x14\n" +
	"\x05bssid\x18\x02 \x01(\tR\x05bssidB%Z#github.com/sagernet/sing-box/daemonb\x06proto3"

var (
	file_daemon_helper_proto_rawDescOnce sync.Once
	file_daemon_helper_proto_rawDescData []byte
)

func file_daemon_helper_proto_rawDescGZIP() []byte {
	file_daemon_helper_proto_rawDescOnce.Do(func() {
		file_daemon_helper_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_daemon_helper_proto_rawDesc), len(file_daemon_helper_proto_rawDesc)))
	})
	return file_daemon_helper_proto_rawDescData
}

var (
	file_daemon_helper_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
	file_daemon_helper_proto_goTypes  = []any{
		(*SubscribeHelperRequestRequest)(nil), // 0: daemon.SubscribeHelperRequestRequest
		(*HelperRequest)(nil),                 // 1: daemon.HelperRequest
		(*FindConnectionOwnerRequest)(nil),    // 2: daemon.FindConnectionOwnerRequest
		(*Notification)(nil),                  // 3: daemon.Notification
		(*HelperResponse)(nil),                // 4: daemon.HelperResponse
		(*ConnectionOwner)(nil),               // 5: daemon.ConnectionOwner
		(*WIFIState)(nil),                     // 6: daemon.WIFIState
		(*emptypb.Empty)(nil),                 // 7: google.protobuf.Empty
	}
)

var file_daemon_helper_proto_depIdxs = []int32{
	7, // 0: daemon.HelperRequest.getWIFIState:type_name -> google.protobuf.Empty
	2, // 1: daemon.HelperRequest.findConnectionOwner:type_name -> daemon.FindConnectionOwnerRequest
	3, // 2: daemon.HelperRequest.sendNotification:type_name -> daemon.Notification
	6, // 3: daemon.HelperResponse.wifiState:type_name -> daemon.WIFIState
	5, // 4: daemon.HelperResponse.connectionOwner:type_name -> daemon.ConnectionOwner
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_daemon_helper_proto_init() }
func file_daemon_helper_proto_init() {
	if File_daemon_helper_proto != nil {
		return
	}
	file_daemon_helper_proto_msgTypes[1].OneofWrappers = []any{
		(*HelperRequest_GetWIFIState)(nil),
		(*HelperRequest_FindConnectionOwner)(nil),
		(*HelperRequest_SendNotification)(nil),
	}
	file_daemon_helper_proto_msgTypes[4].OneofWrappers = []any{
		(*HelperResponse_WifiState)(nil),
		(*HelperResponse_Error)(nil),
		(*HelperResponse_ConnectionOwner)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_daemon_helper_proto_rawDesc), len(file_daemon_helper_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_daemon_helper_proto_goTypes,
		DependencyIndexes: file_daemon_helper_proto_depIdxs,
		MessageInfos:      file_daemon_helper_proto_msgTypes,
	}.Build()
	File_daemon_helper_proto = out.File
	file_daemon_helper_proto_goTypes = nil
	file_daemon_helper_proto_depIdxs = nil
}
