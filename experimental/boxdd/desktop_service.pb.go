package main

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	daemon "github.com/sagernet/sing-box/daemon"

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

type DaemonOwnership int32

const (
	DaemonOwnership_DAEMON_OWNERSHIP_UNSPECIFIED DaemonOwnership = 0
	DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE   DaemonOwnership = 1
	DaemonOwnership_DAEMON_OWNERSHIP_CALLER      DaemonOwnership = 2
	DaemonOwnership_DAEMON_OWNERSHIP_OTHER       DaemonOwnership = 3
)

// Enum value maps for DaemonOwnership.
var (
	DaemonOwnership_name = map[int32]string{
		0: "DAEMON_OWNERSHIP_UNSPECIFIED",
		1: "DAEMON_OWNERSHIP_AVAILABLE",
		2: "DAEMON_OWNERSHIP_CALLER",
		3: "DAEMON_OWNERSHIP_OTHER",
	}
	DaemonOwnership_value = map[string]int32{
		"DAEMON_OWNERSHIP_UNSPECIFIED": 0,
		"DAEMON_OWNERSHIP_AVAILABLE":   1,
		"DAEMON_OWNERSHIP_CALLER":      2,
		"DAEMON_OWNERSHIP_OTHER":       3,
	}
)

func (x DaemonOwnership) Enum() *DaemonOwnership {
	p := new(DaemonOwnership)
	*p = x
	return p
}

func (x DaemonOwnership) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DaemonOwnership) Descriptor() protoreflect.EnumDescriptor {
	return file_experimental_boxdd_desktop_service_proto_enumTypes[0].Descriptor()
}

func (DaemonOwnership) Type() protoreflect.EnumType {
	return &file_experimental_boxdd_desktop_service_proto_enumTypes[0]
}

func (x DaemonOwnership) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DaemonOwnership.Descriptor instead.
func (DaemonOwnership) EnumDescriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{0}
}

type InstallUpdateResult int32

const (
	InstallUpdateResult_INSTALL_UPDATE_RESULT_UNSPECIFIED     InstallUpdateResult = 0
	InstallUpdateResult_INSTALL_UPDATE_RESULT_STARTED         InstallUpdateResult = 1
	InstallUpdateResult_INSTALL_UPDATE_RESULT_SIGNER_MISMATCH InstallUpdateResult = 2
	InstallUpdateResult_INSTALL_UPDATE_RESULT_NOT_NEWER       InstallUpdateResult = 3
)

// Enum value maps for InstallUpdateResult.
var (
	InstallUpdateResult_name = map[int32]string{
		0: "INSTALL_UPDATE_RESULT_UNSPECIFIED",
		1: "INSTALL_UPDATE_RESULT_STARTED",
		2: "INSTALL_UPDATE_RESULT_SIGNER_MISMATCH",
		3: "INSTALL_UPDATE_RESULT_NOT_NEWER",
	}
	InstallUpdateResult_value = map[string]int32{
		"INSTALL_UPDATE_RESULT_UNSPECIFIED":     0,
		"INSTALL_UPDATE_RESULT_STARTED":         1,
		"INSTALL_UPDATE_RESULT_SIGNER_MISMATCH": 2,
		"INSTALL_UPDATE_RESULT_NOT_NEWER":       3,
	}
)

func (x InstallUpdateResult) Enum() *InstallUpdateResult {
	p := new(InstallUpdateResult)
	*p = x
	return p
}

func (x InstallUpdateResult) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (InstallUpdateResult) Descriptor() protoreflect.EnumDescriptor {
	return file_experimental_boxdd_desktop_service_proto_enumTypes[1].Descriptor()
}

func (InstallUpdateResult) Type() protoreflect.EnumType {
	return &file_experimental_boxdd_desktop_service_proto_enumTypes[1]
}

func (x InstallUpdateResult) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use InstallUpdateResult.Descriptor instead.
func (InstallUpdateResult) EnumDescriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{1}
}

type ProfileContent_Type int32

const (
	ProfileContent_LOCAL  ProfileContent_Type = 0
	ProfileContent_ICLOUD ProfileContent_Type = 1
	ProfileContent_REMOTE ProfileContent_Type = 2
)

// Enum value maps for ProfileContent_Type.
var (
	ProfileContent_Type_name = map[int32]string{
		0: "LOCAL",
		1: "ICLOUD",
		2: "REMOTE",
	}
	ProfileContent_Type_value = map[string]int32{
		"LOCAL":  0,
		"ICLOUD": 1,
		"REMOTE": 2,
	}
)

func (x ProfileContent_Type) Enum() *ProfileContent_Type {
	p := new(ProfileContent_Type)
	*p = x
	return p
}

func (x ProfileContent_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ProfileContent_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_experimental_boxdd_desktop_service_proto_enumTypes[2].Descriptor()
}

func (ProfileContent_Type) Type() protoreflect.EnumType {
	return &file_experimental_boxdd_desktop_service_proto_enumTypes[2]
}

func (x ProfileContent_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ProfileContent_Type.Descriptor instead.
func (ProfileContent_Type) EnumDescriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{7, 0}
}

type ArchiveReportRequest struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	SourcePath      string                 `protobuf:"bytes,1,opt,name=source_path,json=sourcePath,proto3" json:"source_path,omitempty"`
	DestinationPath string                 `protobuf:"bytes,2,opt,name=destination_path,json=destinationPath,proto3" json:"destination_path,omitempty"`
	Encrypt         bool                   `protobuf:"varint,3,opt,name=encrypt,proto3" json:"encrypt,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *ArchiveReportRequest) Reset() {
	*x = ArchiveReportRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ArchiveReportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ArchiveReportRequest) ProtoMessage() {}

func (x *ArchiveReportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ArchiveReportRequest.ProtoReflect.Descriptor instead.
func (*ArchiveReportRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{0}
}

func (x *ArchiveReportRequest) GetSourcePath() string {
	if x != nil {
		return x.SourcePath
	}
	return ""
}

func (x *ArchiveReportRequest) GetDestinationPath() string {
	if x != nil {
		return x.DestinationPath
	}
	return ""
}

func (x *ArchiveReportRequest) GetEncrypt() bool {
	if x != nil {
		return x.Encrypt
	}
	return false
}

type StandaloneNetworkQualityTestRequest struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	ConfigUrl         string                 `protobuf:"bytes,1,opt,name=config_url,json=configUrl,proto3" json:"config_url,omitempty"`
	Serial            bool                   `protobuf:"varint,2,opt,name=serial,proto3" json:"serial,omitempty"`
	MaxRuntimeSeconds int32                  `protobuf:"varint,3,opt,name=max_runtime_seconds,json=maxRuntimeSeconds,proto3" json:"max_runtime_seconds,omitempty"`
	Http3             bool                   `protobuf:"varint,4,opt,name=http3,proto3" json:"http3,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *StandaloneNetworkQualityTestRequest) Reset() {
	*x = StandaloneNetworkQualityTestRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StandaloneNetworkQualityTestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StandaloneNetworkQualityTestRequest) ProtoMessage() {}

func (x *StandaloneNetworkQualityTestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StandaloneNetworkQualityTestRequest.ProtoReflect.Descriptor instead.
func (*StandaloneNetworkQualityTestRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{1}
}

func (x *StandaloneNetworkQualityTestRequest) GetConfigUrl() string {
	if x != nil {
		return x.ConfigUrl
	}
	return ""
}

func (x *StandaloneNetworkQualityTestRequest) GetSerial() bool {
	if x != nil {
		return x.Serial
	}
	return false
}

func (x *StandaloneNetworkQualityTestRequest) GetMaxRuntimeSeconds() int32 {
	if x != nil {
		return x.MaxRuntimeSeconds
	}
	return 0
}

func (x *StandaloneNetworkQualityTestRequest) GetHttp3() bool {
	if x != nil {
		return x.Http3
	}
	return false
}

type StandaloneSTUNTestRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Server        string                 `protobuf:"bytes,1,opt,name=server,proto3" json:"server,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StandaloneSTUNTestRequest) Reset() {
	*x = StandaloneSTUNTestRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StandaloneSTUNTestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StandaloneSTUNTestRequest) ProtoMessage() {}

func (x *StandaloneSTUNTestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StandaloneSTUNTestRequest.ProtoReflect.Descriptor instead.
func (*StandaloneSTUNTestRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{2}
}

func (x *StandaloneSTUNTestRequest) GetServer() string {
	if x != nil {
		return x.Server
	}
	return ""
}

type DaemonInfo struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Version       string                 `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	Ownership     DaemonOwnership        `protobuf:"varint,2,opt,name=ownership,proto3,enum=desktop.DaemonOwnership" json:"ownership,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DaemonInfo) Reset() {
	*x = DaemonInfo{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DaemonInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DaemonInfo) ProtoMessage() {}

func (x *DaemonInfo) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DaemonInfo.ProtoReflect.Descriptor instead.
func (*DaemonInfo) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{3}
}

func (x *DaemonInfo) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *DaemonInfo) GetOwnership() DaemonOwnership {
	if x != nil {
		return x.Ownership
	}
	return DaemonOwnership_DAEMON_OWNERSHIP_UNSPECIFIED
}

type StartServiceRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ConfigContent string                 `protobuf:"bytes,1,opt,name=config_content,json=configContent,proto3" json:"config_content,omitempty"`
	Options       *StartOptions          `protobuf:"bytes,2,opt,name=options,proto3" json:"options,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StartServiceRequest) Reset() {
	*x = StartServiceRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StartServiceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartServiceRequest) ProtoMessage() {}

func (x *StartServiceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartServiceRequest.ProtoReflect.Descriptor instead.
func (*StartServiceRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{4}
}

func (x *StartServiceRequest) GetConfigContent() string {
	if x != nil {
		return x.ConfigContent
	}
	return ""
}

func (x *StartServiceRequest) GetOptions() *StartOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

type StartOptions struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	OomKillerEnabled  bool                   `protobuf:"varint,1,opt,name=oom_killer_enabled,json=oomKillerEnabled,proto3" json:"oom_killer_enabled,omitempty"`
	OomKillerDisabled bool                   `protobuf:"varint,2,opt,name=oom_killer_disabled,json=oomKillerDisabled,proto3" json:"oom_killer_disabled,omitempty"`
	OomMemoryLimit    int64                  `protobuf:"varint,3,opt,name=oom_memory_limit,json=oomMemoryLimit,proto3" json:"oom_memory_limit,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *StartOptions) Reset() {
	*x = StartOptions{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StartOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartOptions) ProtoMessage() {}

func (x *StartOptions) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartOptions.ProtoReflect.Descriptor instead.
func (*StartOptions) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{5}
}

func (x *StartOptions) GetOomKillerEnabled() bool {
	if x != nil {
		return x.OomKillerEnabled
	}
	return false
}

func (x *StartOptions) GetOomKillerDisabled() bool {
	if x != nil {
		return x.OomKillerDisabled
	}
	return false
}

func (x *StartOptions) GetOomMemoryLimit() int64 {
	if x != nil {
		return x.OomMemoryLimit
	}
	return 0
}

type ConfigContent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Content       string                 `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConfigContent) Reset() {
	*x = ConfigContent{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConfigContent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConfigContent) ProtoMessage() {}

func (x *ConfigContent) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConfigContent.ProtoReflect.Descriptor instead.
func (*ConfigContent) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{6}
}

func (x *ConfigContent) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

type ProfileContent struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	Type               ProfileContent_Type    `protobuf:"varint,1,opt,name=type,proto3,enum=desktop.ProfileContent_Type" json:"type,omitempty"`
	Name               string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Config             string                 `protobuf:"bytes,3,opt,name=config,proto3" json:"config,omitempty"`
	RemotePath         string                 `protobuf:"bytes,4,opt,name=remote_path,json=remotePath,proto3" json:"remote_path,omitempty"`
	AutoUpdate         bool                   `protobuf:"varint,5,opt,name=auto_update,json=autoUpdate,proto3" json:"auto_update,omitempty"`
	AutoUpdateInterval int32                  `protobuf:"varint,6,opt,name=auto_update_interval,json=autoUpdateInterval,proto3" json:"auto_update_interval,omitempty"`
	LastUpdated        int64                  `protobuf:"varint,7,opt,name=last_updated,json=lastUpdated,proto3" json:"last_updated,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *ProfileContent) Reset() {
	*x = ProfileContent{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProfileContent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProfileContent) ProtoMessage() {}

func (x *ProfileContent) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProfileContent.ProtoReflect.Descriptor instead.
func (*ProfileContent) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{7}
}

func (x *ProfileContent) GetType() ProfileContent_Type {
	if x != nil {
		return x.Type
	}
	return ProfileContent_LOCAL
}

func (x *ProfileContent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ProfileContent) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *ProfileContent) GetRemotePath() string {
	if x != nil {
		return x.RemotePath
	}
	return ""
}

func (x *ProfileContent) GetAutoUpdate() bool {
	if x != nil {
		return x.AutoUpdate
	}
	return false
}

func (x *ProfileContent) GetAutoUpdateInterval() int32 {
	if x != nil {
		return x.AutoUpdateInterval
	}
	return 0
}

func (x *ProfileContent) GetLastUpdated() int64 {
	if x != nil {
		return x.LastUpdated
	}
	return 0
}

type ProfileData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Data          []byte                 `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProfileData) Reset() {
	*x = ProfileData{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProfileData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProfileData) ProtoMessage() {}

func (x *ProfileData) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProfileData.ProtoReflect.Descriptor instead.
func (*ProfileData) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{8}
}

func (x *ProfileData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type WorkingDirectoryInfo struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Path          string                 `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Size          int64                  `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WorkingDirectoryInfo) Reset() {
	*x = WorkingDirectoryInfo{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WorkingDirectoryInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WorkingDirectoryInfo) ProtoMessage() {}

func (x *WorkingDirectoryInfo) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WorkingDirectoryInfo.ProtoReflect.Descriptor instead.
func (*WorkingDirectoryInfo) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{9}
}

func (x *WorkingDirectoryInfo) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *WorkingDirectoryInfo) GetSize() int64 {
	if x != nil {
		return x.Size
	}
	return 0
}

type CrashReportList struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Reports       []*CrashReportEntry    `protobuf:"bytes,1,rep,name=reports,proto3" json:"reports,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportList) Reset() {
	*x = CrashReportList{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportList) ProtoMessage() {}

func (x *CrashReportList) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportList.ProtoReflect.Descriptor instead.
func (*CrashReportList) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{10}
}

func (x *CrashReportList) GetReports() []*CrashReportEntry {
	if x != nil {
		return x.Reports
	}
	return nil
}

type CrashReportEntry struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	CrashedAt     int64                  `protobuf:"varint,2,opt,name=crashed_at,json=crashedAt,proto3" json:"crashed_at,omitempty"`
	IsRead        bool                   `protobuf:"varint,3,opt,name=is_read,json=isRead,proto3" json:"is_read,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportEntry) Reset() {
	*x = CrashReportEntry{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportEntry) ProtoMessage() {}

func (x *CrashReportEntry) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportEntry.ProtoReflect.Descriptor instead.
func (*CrashReportEntry) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{11}
}

func (x *CrashReportEntry) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CrashReportEntry) GetCrashedAt() int64 {
	if x != nil {
		return x.CrashedAt
	}
	return 0
}

func (x *CrashReportEntry) GetIsRead() bool {
	if x != nil {
		return x.IsRead
	}
	return false
}

type CrashReportRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportRequest) Reset() {
	*x = CrashReportRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportRequest) ProtoMessage() {}

func (x *CrashReportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportRequest.ProtoReflect.Descriptor instead.
func (*CrashReportRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{12}
}

func (x *CrashReportRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type CrashReportExportRequest struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Name              string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	WithConfiguration bool                   `protobuf:"varint,2,opt,name=with_configuration,json=withConfiguration,proto3" json:"with_configuration,omitempty"`
	WithLog           bool                   `protobuf:"varint,3,opt,name=with_log,json=withLog,proto3" json:"with_log,omitempty"`
	Encrypt           bool                   `protobuf:"varint,4,opt,name=encrypt,proto3" json:"encrypt,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *CrashReportExportRequest) Reset() {
	*x = CrashReportExportRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[13]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportExportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportExportRequest) ProtoMessage() {}

func (x *CrashReportExportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[13]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportExportRequest.ProtoReflect.Descriptor instead.
func (*CrashReportExportRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{13}
}

func (x *CrashReportExportRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CrashReportExportRequest) GetWithConfiguration() bool {
	if x != nil {
		return x.WithConfiguration
	}
	return false
}

func (x *CrashReportExportRequest) GetWithLog() bool {
	if x != nil {
		return x.WithLog
	}
	return false
}

func (x *CrashReportExportRequest) GetEncrypt() bool {
	if x != nil {
		return x.Encrypt
	}
	return false
}

type CrashReportContent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Files         []*CrashReportFile     `protobuf:"bytes,1,rep,name=files,proto3" json:"files,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportContent) Reset() {
	*x = CrashReportContent{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[14]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportContent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportContent) ProtoMessage() {}

func (x *CrashReportContent) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[14]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportContent.ProtoReflect.Descriptor instead.
func (*CrashReportContent) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{14}
}

func (x *CrashReportContent) GetFiles() []*CrashReportFile {
	if x != nil {
		return x.Files
	}
	return nil
}

type CrashReportFile struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Content       string                 `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportFile) Reset() {
	*x = CrashReportFile{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[15]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportFile) ProtoMessage() {}

func (x *CrashReportFile) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[15]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportFile.ProtoReflect.Descriptor instead.
func (*CrashReportFile) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{15}
}

func (x *CrashReportFile) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CrashReportFile) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

type CrashReportArchive struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FileName      string                 `protobuf:"bytes,1,opt,name=file_name,json=fileName,proto3" json:"file_name,omitempty"`
	Data          []byte                 `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CrashReportArchive) Reset() {
	*x = CrashReportArchive{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[16]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CrashReportArchive) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CrashReportArchive) ProtoMessage() {}

func (x *CrashReportArchive) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[16]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CrashReportArchive.ProtoReflect.Descriptor instead.
func (*CrashReportArchive) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{16}
}

func (x *CrashReportArchive) GetFileName() string {
	if x != nil {
		return x.FileName
	}
	return ""
}

func (x *CrashReportArchive) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type OOMReportList struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Reports       []*OOMReportEntry      `protobuf:"bytes,1,rep,name=reports,proto3" json:"reports,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OOMReportList) Reset() {
	*x = OOMReportList{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[17]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportList) ProtoMessage() {}

func (x *OOMReportList) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[17]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportList.ProtoReflect.Descriptor instead.
func (*OOMReportList) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{17}
}

func (x *OOMReportList) GetReports() []*OOMReportEntry {
	if x != nil {
		return x.Reports
	}
	return nil
}

type OOMReportEntry struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	RecordedAt    int64                  `protobuf:"varint,2,opt,name=recorded_at,json=recordedAt,proto3" json:"recorded_at,omitempty"`
	IsRead        bool                   `protobuf:"varint,3,opt,name=is_read,json=isRead,proto3" json:"is_read,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OOMReportEntry) Reset() {
	*x = OOMReportEntry{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[18]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportEntry) ProtoMessage() {}

func (x *OOMReportEntry) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[18]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportEntry.ProtoReflect.Descriptor instead.
func (*OOMReportEntry) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{18}
}

func (x *OOMReportEntry) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *OOMReportEntry) GetRecordedAt() int64 {
	if x != nil {
		return x.RecordedAt
	}
	return 0
}

func (x *OOMReportEntry) GetIsRead() bool {
	if x != nil {
		return x.IsRead
	}
	return false
}

type OOMReportRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OOMReportRequest) Reset() {
	*x = OOMReportRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[19]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportRequest) ProtoMessage() {}

func (x *OOMReportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[19]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportRequest.ProtoReflect.Descriptor instead.
func (*OOMReportRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{19}
}

func (x *OOMReportRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type OOMReportExportRequest struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Name              string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	WithConfiguration bool                   `protobuf:"varint,2,opt,name=with_configuration,json=withConfiguration,proto3" json:"with_configuration,omitempty"`
	WithLog           bool                   `protobuf:"varint,3,opt,name=with_log,json=withLog,proto3" json:"with_log,omitempty"`
	Encrypt           bool                   `protobuf:"varint,4,opt,name=encrypt,proto3" json:"encrypt,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *OOMReportExportRequest) Reset() {
	*x = OOMReportExportRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[20]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportExportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportExportRequest) ProtoMessage() {}

func (x *OOMReportExportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[20]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportExportRequest.ProtoReflect.Descriptor instead.
func (*OOMReportExportRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{20}
}

func (x *OOMReportExportRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *OOMReportExportRequest) GetWithConfiguration() bool {
	if x != nil {
		return x.WithConfiguration
	}
	return false
}

func (x *OOMReportExportRequest) GetWithLog() bool {
	if x != nil {
		return x.WithLog
	}
	return false
}

func (x *OOMReportExportRequest) GetEncrypt() bool {
	if x != nil {
		return x.Encrypt
	}
	return false
}

type OOMReportContent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Files         []*OOMReportFile       `protobuf:"bytes,1,rep,name=files,proto3" json:"files,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OOMReportContent) Reset() {
	*x = OOMReportContent{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[21]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportContent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportContent) ProtoMessage() {}

func (x *OOMReportContent) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[21]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportContent.ProtoReflect.Descriptor instead.
func (*OOMReportContent) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{21}
}

func (x *OOMReportContent) GetFiles() []*OOMReportFile {
	if x != nil {
		return x.Files
	}
	return nil
}

type OOMReportFile struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Content       []byte                 `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
	IsProfile     bool                   `protobuf:"varint,3,opt,name=is_profile,json=isProfile,proto3" json:"is_profile,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *OOMReportFile) Reset() {
	*x = OOMReportFile{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[22]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OOMReportFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OOMReportFile) ProtoMessage() {}

func (x *OOMReportFile) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[22]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OOMReportFile.ProtoReflect.Descriptor instead.
func (*OOMReportFile) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{22}
}

func (x *OOMReportFile) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *OOMReportFile) GetContent() []byte {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *OOMReportFile) GetIsProfile() bool {
	if x != nil {
		return x.IsProfile
	}
	return false
}

type InstallUpdateRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	InstallerPath string                 `protobuf:"bytes,1,opt,name=installer_path,json=installerPath,proto3" json:"installer_path,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *InstallUpdateRequest) Reset() {
	*x = InstallUpdateRequest{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[23]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InstallUpdateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InstallUpdateRequest) ProtoMessage() {}

func (x *InstallUpdateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[23]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InstallUpdateRequest.ProtoReflect.Descriptor instead.
func (*InstallUpdateRequest) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{23}
}

func (x *InstallUpdateRequest) GetInstallerPath() string {
	if x != nil {
		return x.InstallerPath
	}
	return ""
}

type InstallUpdateResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Result        InstallUpdateResult    `protobuf:"varint,1,opt,name=result,proto3,enum=desktop.InstallUpdateResult" json:"result,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *InstallUpdateResponse) Reset() {
	*x = InstallUpdateResponse{}
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[24]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InstallUpdateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InstallUpdateResponse) ProtoMessage() {}

func (x *InstallUpdateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_experimental_boxdd_desktop_service_proto_msgTypes[24]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InstallUpdateResponse.ProtoReflect.Descriptor instead.
func (*InstallUpdateResponse) Descriptor() ([]byte, []int) {
	return file_experimental_boxdd_desktop_service_proto_rawDescGZIP(), []int{24}
}

func (x *InstallUpdateResponse) GetResult() InstallUpdateResult {
	if x != nil {
		return x.Result
	}
	return InstallUpdateResult_INSTALL_UPDATE_RESULT_UNSPECIFIED
}

var File_experimental_boxdd_desktop_service_proto protoreflect.FileDescriptor

const file_experimental_boxdd_desktop_service_proto_rawDesc = "" +
	"\n" +
	"(experimental/boxdd/desktop_service.proto\x12\adesktop\x1a\x1bgoogle/protobuf/empty.proto\x1a\x1cdaemon/started_service.proto\"|\n" +
	"\x14ArchiveReportRequest\x12\x1f\n" +
	"\vsource_path\x18\x01 \x01(\tR\n" +
	"sourcePath\x12)\n" +
	"\x10destination_path\x18\x02 \x01(\tR\x0fdestinationPath\x12\x18\n" +
	"\aencrypt\x18\x03 \x01(\bR\aencrypt\"\xa2\x01\n" +
	"#StandaloneNetworkQualityTestRequest\x12\x1d\n" +
	"\n" +
	"config_url\x18\x01 \x01(\tR\tconfigUrl\x12\x16\n" +
	"\x06serial\x18\x02 \x01(\bR\x06serial\x12.\n" +
	"\x13max_runtime_seconds\x18\x03 \x01(\x05R\x11maxRuntimeSeconds\x12\x14\n" +
	"\x05http3\x18\x04 \x01(\bR\x05http3\"3\n" +
	"\x19StandaloneSTUNTestRequest\x12\x16\n" +
	"\x06server\x18\x01 \x01(\tR\x06server\"^\n" +
	"\n" +
	"DaemonInfo\x12\x18\n" +
	"\aversion\x18\x01 \x01(\tR\aversion\x126\n" +
	"\townership\x18\x02 \x01(\x0e2\x18.desktop.DaemonOwnershipR\townership\"m\n" +
	"\x13StartServiceRequest\x12%\n" +
	"\x0econfig_content\x18\x01 \x01(\tR\rconfigContent\x12/\n" +
	"\aoptions\x18\x02 \x01(\v2\x15.desktop.StartOptionsR\aoptions\"\x96\x01\n" +
	"\fStartOptions\x12,\n" +
	"\x12oom_killer_enabled\x18\x01 \x01(\bR\x10oomKillerEnabled\x12.\n" +
	"\x13oom_killer_disabled\x18\x02 \x01(\bR\x11oomKillerDisabled\x12(\n" +
	"\x10oom_memory_limit\x18\x03 \x01(\x03R\x0eoomMemoryLimit\")\n" +
	"\rConfigContent\x12\x18\n" +
	"\acontent\x18\x01 \x01(\tR\acontent\"\xb0\x02\n" +
	"\x0eProfileContent\x120\n" +
	"\x04type\x18\x01 \x01(\x0e2\x1c.desktop.ProfileContent.TypeR\x04type\x12\x12\n" +
	"\x04name\x18\x02 \x01(\tR\x04name\x12\x16\n" +
	"\x06config\x18\x03 \x01(\tR\x06config\x12\x1f\n" +
	"\vremote_path\x18\x04 \x01(\tR\n" +
	"remotePath\x12\x1f\n" +
	"\vauto_update\x18\x05 \x01(\bR\n" +
	"autoUpdate\x120\n" +
	"\x14auto_update_interval\x18\x06 \x01(\x05R\x12autoUpdateInterval\x12!\n" +
	"\flast_updated\x18\a \x01(\x03R\vlastUpdated\")\n" +
	"\x04Type\x12\t\n" +
	"\x05LOCAL\x10\x00\x12\n" +
	"\n" +
	"\x06ICLOUD\x10\x01\x12\n" +
	"\n" +
	"\x06REMOTE\x10\x02\"!\n" +
	"\vProfileData\x12\x12\n" +
	"\x04data\x18\x01 \x01(\fR\x04data\">\n" +
	"\x14WorkingDirectoryInfo\x12\x12\n" +
	"\x04path\x18\x01 \x01(\tR\x04path\x12\x12\n" +
	"\x04size\x18\x02 \x01(\x03R\x04size\"F\n" +
	"\x0fCrashReportList\x123\n" +
	"\areports\x18\x01 \x03(\v2\x19.desktop.CrashReportEntryR\areports\"^\n" +
	"\x10CrashReportEntry\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x1d\n" +
	"\n" +
	"crashed_at\x18\x02 \x01(\x03R\tcrashedAt\x12\x17\n" +
	"\ais_read\x18\x03 \x01(\bR\x06isRead\"(\n" +
	"\x12CrashReportRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"\x92\x01\n" +
	"\x18CrashReportExportRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12-\n" +
	"\x12with_configuration\x18\x02 \x01(\bR\x11withConfiguration\x12\x19\n" +
	"\bwith_log\x18\x03 \x01(\bR\awithLog\x12\x18\n" +
	"\aencrypt\x18\x04 \x01(\bR\aencrypt\"D\n" +
	"\x12CrashReportContent\x12.\n" +
	"\x05files\x18\x01 \x03(\v2\x18.desktop.CrashReportFileR\x05files\"?\n" +
	"\x0fCrashReportFile\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x18\n" +
	"\acontent\x18\x02 \x01(\tR\acontent\"E\n" +
	"\x12CrashReportArchive\x12\x1b\n" +
	"\tfile_name\x18\x01 \x01(\tR\bfileName\x12\x12\n" +
	"\x04data\x18\x02 \x01(\fR\x04data\"B\n" +
	"\rOOMReportList\x121\n" +
	"\areports\x18\x01 \x03(\v2\x17.desktop.OOMReportEntryR\areports\"^\n" +
	"\x0eOOMReportEntry\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x1f\n" +
	"\vrecorded_at\x18\x02 \x01(\x03R\n" +
	"recordedAt\x12\x17\n" +
	"\ais_read\x18\x03 \x01(\bR\x06isRead\"&\n" +
	"\x10OOMReportRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"\x90\x01\n" +
	"\x16OOMReportExportRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12-\n" +
	"\x12with_configuration\x18\x02 \x01(\bR\x11withConfiguration\x12\x19\n" +
	"\bwith_log\x18\x03 \x01(\bR\awithLog\x12\x18\n" +
	"\aencrypt\x18\x04 \x01(\bR\aencrypt\"@\n" +
	"\x10OOMReportContent\x12,\n" +
	"\x05files\x18\x01 \x03(\v2\x16.desktop.OOMReportFileR\x05files\"\\\n" +
	"\rOOMReportFile\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x18\n" +
	"\acontent\x18\x02 \x01(\fR\acontent\x12\x1d\n" +
	"\n" +
	"is_profile\x18\x03 \x01(\bR\tisProfile\"=\n" +
	"\x14InstallUpdateRequest\x12%\n" +
	"\x0einstaller_path\x18\x01 \x01(\tR\rinstallerPath\"M\n" +
	"\x15InstallUpdateResponse\x124\n" +
	"\x06result\x18\x01 \x01(\x0e2\x1c.desktop.InstallUpdateResultR\x06result*\x8c\x01\n" +
	"\x0fDaemonOwnership\x12 \n" +
	"\x1cDAEMON_OWNERSHIP_UNSPECIFIED\x10\x00\x12\x1e\n" +
	"\x1aDAEMON_OWNERSHIP_AVAILABLE\x10\x01\x12\x1b\n" +
	"\x17DAEMON_OWNERSHIP_CALLER\x10\x02\x12\x1a\n" +
	"\x16DAEMON_OWNERSHIP_OTHER\x10\x03*\xaf\x01\n" +
	"\x13InstallUpdateResult\x12%\n" +
	"!INSTALL_UPDATE_RESULT_UNSPECIFIED\x10\x00\x12!\n" +
	"\x1dINSTALL_UPDATE_RESULT_STARTED\x10\x01\x12)\n" +
	"%INSTALL_UPDATE_RESULT_SIGNER_MISMATCH\x10\x02\x12#\n" +
	"\x1fINSTALL_UPDATE_RESULT_NOT_NEWER\x10\x032\x9c\v\n" +
	"\x0eDesktopService\x12>\n" +
	"\rGetDaemonInfo\x12\x16.google.protobuf.Empty\x1a\x13.desktop.DaemonInfo\"\x00\x12@\n" +
	"\fClaimService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12C\n" +
	"\x0fTakeOverService\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12F\n" +
	"\fStartService\x12\x1c.desktop.StartServiceRequest\x1a\x16.google.protobuf.Empty\"\x00\x12N\n" +
	"\x13GetWorkingDirectory\x12\x16.google.protobuf.Empty\x1a\x1d.desktop.WorkingDirectoryInfo\"\x00\x12K\n" +
	"\x17DestroyWorkingDirectory\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12F\n" +
	"\x10ListCrashReports\x12\x16.google.protobuf.Empty\x1a\x18.desktop.CrashReportList\"\x00\x12M\n" +
	"\x0fReadCrashReport\x12\x1b.desktop.CrashReportRequest\x1a\x1b.desktop.CrashReportContent\"\x00\x12L\n" +
	"\x13MarkCrashReportRead\x12\x1b.desktop.CrashReportRequest\x1a\x16.google.protobuf.Empty\"\x00\x12U\n" +
	"\x11ExportCrashReport\x12!.desktop.CrashReportExportRequest\x1a\x1b.desktop.CrashReportArchive\"\x00\x12J\n" +
	"\x11DeleteCrashReport\x12\x1b.desktop.CrashReportRequest\x1a\x16.google.protobuf.Empty\"\x00\x12I\n" +
	"\x15DeleteAllCrashReports\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12B\n" +
	"\x0eListOOMReports\x12\x16.google.protobuf.Empty\x1a\x16.desktop.OOMReportList\"\x00\x12G\n" +
	"\rReadOOMReport\x12\x19.desktop.OOMReportRequest\x1a\x19.desktop.OOMReportContent\"\x00\x12H\n" +
	"\x11MarkOOMReportRead\x12\x19.desktop.OOMReportRequest\x1a\x16.google.protobuf.Empty\"\x00\x12Q\n" +
	"\x0fExportOOMReport\x12\x1f.desktop.OOMReportExportRequest\x1a\x1b.desktop.CrashReportArchive\"\x00\x12F\n" +
	"\x0fDeleteOOMReport\x12\x19.desktop.OOMReportRequest\x1a\x16.google.protobuf.Empty\"\x00\x12G\n" +
	"\x13DeleteAllOOMReports\x12\x16.google.protobuf.Empty\x1a\x16.google.protobuf.Empty\"\x00\x12P\n" +
	"\rInstallUpdate\x12\x1d.desktop.InstallUpdateRequest\x1a\x1e.desktop.InstallUpdateResponse\"\x002\xbd\x04\n" +
	"\x12ApplicationService\x12?\n" +
	"\vCheckConfig\x12\x16.desktop.ConfigContent\x1a\x16.google.protobuf.Empty\"\x00\x12@\n" +
	"\fFormatConfig\x12\x16.desktop.ConfigContent\x1a\x16.desktop.ConfigContent\"\x00\x12@\n" +
	"\rEncodeProfile\x12\x17.desktop.ProfileContent\x1a\x14.desktop.ProfileData\"\x00\x12@\n" +
	"\rDecodeProfile\x12\x14.desktop.ProfileData\x1a\x17.desktop.ProfileContent\"\x00\x12H\n" +
	"\rArchiveReport\x12\x1d.desktop.ArchiveReportRequest\x1a\x16.google.protobuf.Empty\"\x00\x12y\n" +
	"!StartStandaloneNetworkQualityTest\x12,.desktop.StandaloneNetworkQualityTestRequest\x1a\".daemon.NetworkQualityTestProgress\"\x000\x01\x12[\n" +
	"\x17StartStandaloneSTUNTest\x12\".desktop.StandaloneSTUNTestRequest\x1a\x18.daemon.STUNTestProgress\"\x000\x01B6Z4github.com/sagernet/sing-box/experimental/boxdd;mainb\x06proto3"

var (
	file_experimental_boxdd_desktop_service_proto_rawDescOnce sync.Once
	file_experimental_boxdd_desktop_service_proto_rawDescData []byte
)

func file_experimental_boxdd_desktop_service_proto_rawDescGZIP() []byte {
	file_experimental_boxdd_desktop_service_proto_rawDescOnce.Do(func() {
		file_experimental_boxdd_desktop_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_experimental_boxdd_desktop_service_proto_rawDesc), len(file_experimental_boxdd_desktop_service_proto_rawDesc)))
	})
	return file_experimental_boxdd_desktop_service_proto_rawDescData
}

var (
	file_experimental_boxdd_desktop_service_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
	file_experimental_boxdd_desktop_service_proto_msgTypes  = make([]protoimpl.MessageInfo, 25)
	file_experimental_boxdd_desktop_service_proto_goTypes   = []any{
		(DaemonOwnership)(0),                        // 0: desktop.DaemonOwnership
		(InstallUpdateResult)(0),                    // 1: desktop.InstallUpdateResult
		(ProfileContent_Type)(0),                    // 2: desktop.ProfileContent.Type
		(*ArchiveReportRequest)(nil),                // 3: desktop.ArchiveReportRequest
		(*StandaloneNetworkQualityTestRequest)(nil), // 4: desktop.StandaloneNetworkQualityTestRequest
		(*StandaloneSTUNTestRequest)(nil),           // 5: desktop.StandaloneSTUNTestRequest
		(*DaemonInfo)(nil),                          // 6: desktop.DaemonInfo
		(*StartServiceRequest)(nil),                 // 7: desktop.StartServiceRequest
		(*StartOptions)(nil),                        // 8: desktop.StartOptions
		(*ConfigContent)(nil),                       // 9: desktop.ConfigContent
		(*ProfileContent)(nil),                      // 10: desktop.ProfileContent
		(*ProfileData)(nil),                         // 11: desktop.ProfileData
		(*WorkingDirectoryInfo)(nil),                // 12: desktop.WorkingDirectoryInfo
		(*CrashReportList)(nil),                     // 13: desktop.CrashReportList
		(*CrashReportEntry)(nil),                    // 14: desktop.CrashReportEntry
		(*CrashReportRequest)(nil),                  // 15: desktop.CrashReportRequest
		(*CrashReportExportRequest)(nil),            // 16: desktop.CrashReportExportRequest
		(*CrashReportContent)(nil),                  // 17: desktop.CrashReportContent
		(*CrashReportFile)(nil),                     // 18: desktop.CrashReportFile
		(*CrashReportArchive)(nil),                  // 19: desktop.CrashReportArchive
		(*OOMReportList)(nil),                       // 20: desktop.OOMReportList
		(*OOMReportEntry)(nil),                      // 21: desktop.OOMReportEntry
		(*OOMReportRequest)(nil),                    // 22: desktop.OOMReportRequest
		(*OOMReportExportRequest)(nil),              // 23: desktop.OOMReportExportRequest
		(*OOMReportContent)(nil),                    // 24: desktop.OOMReportContent
		(*OOMReportFile)(nil),                       // 25: desktop.OOMReportFile
		(*InstallUpdateRequest)(nil),                // 26: desktop.InstallUpdateRequest
		(*InstallUpdateResponse)(nil),               // 27: desktop.InstallUpdateResponse
		(*emptypb.Empty)(nil),                       // 28: google.protobuf.Empty
		(*daemon.NetworkQualityTestProgress)(nil),   // 29: daemon.NetworkQualityTestProgress
		(*daemon.STUNTestProgress)(nil),             // 30: daemon.STUNTestProgress
	}
)

var file_experimental_boxdd_desktop_service_proto_depIdxs = []int32{
	0,  // 0: desktop.DaemonInfo.ownership:type_name -> desktop.DaemonOwnership
	8,  // 1: desktop.StartServiceRequest.options:type_name -> desktop.StartOptions
	2,  // 2: desktop.ProfileContent.type:type_name -> desktop.ProfileContent.Type
	14, // 3: desktop.CrashReportList.reports:type_name -> desktop.CrashReportEntry
	18, // 4: desktop.CrashReportContent.files:type_name -> desktop.CrashReportFile
	21, // 5: desktop.OOMReportList.reports:type_name -> desktop.OOMReportEntry
	25, // 6: desktop.OOMReportContent.files:type_name -> desktop.OOMReportFile
	1,  // 7: desktop.InstallUpdateResponse.result:type_name -> desktop.InstallUpdateResult
	28, // 8: desktop.DesktopService.GetDaemonInfo:input_type -> google.protobuf.Empty
	28, // 9: desktop.DesktopService.ClaimService:input_type -> google.protobuf.Empty
	28, // 10: desktop.DesktopService.TakeOverService:input_type -> google.protobuf.Empty
	7,  // 11: desktop.DesktopService.StartService:input_type -> desktop.StartServiceRequest
	28, // 12: desktop.DesktopService.GetWorkingDirectory:input_type -> google.protobuf.Empty
	28, // 13: desktop.DesktopService.DestroyWorkingDirectory:input_type -> google.protobuf.Empty
	28, // 14: desktop.DesktopService.ListCrashReports:input_type -> google.protobuf.Empty
	15, // 15: desktop.DesktopService.ReadCrashReport:input_type -> desktop.CrashReportRequest
	15, // 16: desktop.DesktopService.MarkCrashReportRead:input_type -> desktop.CrashReportRequest
	16, // 17: desktop.DesktopService.ExportCrashReport:input_type -> desktop.CrashReportExportRequest
	15, // 18: desktop.DesktopService.DeleteCrashReport:input_type -> desktop.CrashReportRequest
	28, // 19: desktop.DesktopService.DeleteAllCrashReports:input_type -> google.protobuf.Empty
	28, // 20: desktop.DesktopService.ListOOMReports:input_type -> google.protobuf.Empty
	22, // 21: desktop.DesktopService.ReadOOMReport:input_type -> desktop.OOMReportRequest
	22, // 22: desktop.DesktopService.MarkOOMReportRead:input_type -> desktop.OOMReportRequest
	23, // 23: desktop.DesktopService.ExportOOMReport:input_type -> desktop.OOMReportExportRequest
	22, // 24: desktop.DesktopService.DeleteOOMReport:input_type -> desktop.OOMReportRequest
	28, // 25: desktop.DesktopService.DeleteAllOOMReports:input_type -> google.protobuf.Empty
	26, // 26: desktop.DesktopService.InstallUpdate:input_type -> desktop.InstallUpdateRequest
	9,  // 27: desktop.ApplicationService.CheckConfig:input_type -> desktop.ConfigContent
	9,  // 28: desktop.ApplicationService.FormatConfig:input_type -> desktop.ConfigContent
	10, // 29: desktop.ApplicationService.EncodeProfile:input_type -> desktop.ProfileContent
	11, // 30: desktop.ApplicationService.DecodeProfile:input_type -> desktop.ProfileData
	3,  // 31: desktop.ApplicationService.ArchiveReport:input_type -> desktop.ArchiveReportRequest
	4,  // 32: desktop.ApplicationService.StartStandaloneNetworkQualityTest:input_type -> desktop.StandaloneNetworkQualityTestRequest
	5,  // 33: desktop.ApplicationService.StartStandaloneSTUNTest:input_type -> desktop.StandaloneSTUNTestRequest
	6,  // 34: desktop.DesktopService.GetDaemonInfo:output_type -> desktop.DaemonInfo
	28, // 35: desktop.DesktopService.ClaimService:output_type -> google.protobuf.Empty
	28, // 36: desktop.DesktopService.TakeOverService:output_type -> google.protobuf.Empty
	28, // 37: desktop.DesktopService.StartService:output_type -> google.protobuf.Empty
	12, // 38: desktop.DesktopService.GetWorkingDirectory:output_type -> desktop.WorkingDirectoryInfo
	28, // 39: desktop.DesktopService.DestroyWorkingDirectory:output_type -> google.protobuf.Empty
	13, // 40: desktop.DesktopService.ListCrashReports:output_type -> desktop.CrashReportList
	17, // 41: desktop.DesktopService.ReadCrashReport:output_type -> desktop.CrashReportContent
	28, // 42: desktop.DesktopService.MarkCrashReportRead:output_type -> google.protobuf.Empty
	19, // 43: desktop.DesktopService.ExportCrashReport:output_type -> desktop.CrashReportArchive
	28, // 44: desktop.DesktopService.DeleteCrashReport:output_type -> google.protobuf.Empty
	28, // 45: desktop.DesktopService.DeleteAllCrashReports:output_type -> google.protobuf.Empty
	20, // 46: desktop.DesktopService.ListOOMReports:output_type -> desktop.OOMReportList
	24, // 47: desktop.DesktopService.ReadOOMReport:output_type -> desktop.OOMReportContent
	28, // 48: desktop.DesktopService.MarkOOMReportRead:output_type -> google.protobuf.Empty
	19, // 49: desktop.DesktopService.ExportOOMReport:output_type -> desktop.CrashReportArchive
	28, // 50: desktop.DesktopService.DeleteOOMReport:output_type -> google.protobuf.Empty
	28, // 51: desktop.DesktopService.DeleteAllOOMReports:output_type -> google.protobuf.Empty
	27, // 52: desktop.DesktopService.InstallUpdate:output_type -> desktop.InstallUpdateResponse
	28, // 53: desktop.ApplicationService.CheckConfig:output_type -> google.protobuf.Empty
	9,  // 54: desktop.ApplicationService.FormatConfig:output_type -> desktop.ConfigContent
	11, // 55: desktop.ApplicationService.EncodeProfile:output_type -> desktop.ProfileData
	10, // 56: desktop.ApplicationService.DecodeProfile:output_type -> desktop.ProfileContent
	28, // 57: desktop.ApplicationService.ArchiveReport:output_type -> google.protobuf.Empty
	29, // 58: desktop.ApplicationService.StartStandaloneNetworkQualityTest:output_type -> daemon.NetworkQualityTestProgress
	30, // 59: desktop.ApplicationService.StartStandaloneSTUNTest:output_type -> daemon.STUNTestProgress
	34, // [34:60] is the sub-list for method output_type
	8,  // [8:34] is the sub-list for method input_type
	8,  // [8:8] is the sub-list for extension type_name
	8,  // [8:8] is the sub-list for extension extendee
	0,  // [0:8] is the sub-list for field type_name
}

func init() { file_experimental_boxdd_desktop_service_proto_init() }
func file_experimental_boxdd_desktop_service_proto_init() {
	if File_experimental_boxdd_desktop_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_experimental_boxdd_desktop_service_proto_rawDesc), len(file_experimental_boxdd_desktop_service_proto_rawDesc)),
			NumEnums:      3,
			NumMessages:   25,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_experimental_boxdd_desktop_service_proto_goTypes,
		DependencyIndexes: file_experimental_boxdd_desktop_service_proto_depIdxs,
		EnumInfos:         file_experimental_boxdd_desktop_service_proto_enumTypes,
		MessageInfos:      file_experimental_boxdd_desktop_service_proto_msgTypes,
	}.Build()
	File_experimental_boxdd_desktop_service_proto = out.File
	file_experimental_boxdd_desktop_service_proto_goTypes = nil
	file_experimental_boxdd_desktop_service_proto_depIdxs = nil
}
