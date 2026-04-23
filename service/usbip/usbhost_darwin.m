#import "usbhost_darwin.h"

#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>
#import <IOUSBHost/AppleUSBDescriptorParsing.h>
#import <mach/mach_time.h>
#import <stdlib.h>
#import <string.h>

@interface BoxUSBHostDevice : NSObject
@property(nonatomic, strong) IOUSBHostDevice *device;
@property(nonatomic, strong) NSMutableArray<IOUSBHostInterface *> *interfaces;
@property(nonatomic, strong) NSMutableDictionary<NSNumber *, IOUSBHostPipe *> *pipes;
@end

@implementation BoxUSBHostDevice
@end

@interface BoxUSBHostController : NSObject
@property(nonatomic, strong) IOUSBHostControllerInterface *controller;
@property(nonatomic, assign) uintptr_t ref;
@end

@implementation BoxUSBHostController
@end

struct box_usbhost_device {
	void *object;
};

struct box_usbhost_controller {
	void *object;
};

struct box_usbhost_device_sm {
	void *object;
};

struct box_usbhost_endpoint_sm {
	void *object;
};

static BoxUSBHostDevice *box_device(box_usbhost_device_t *device) {
	return device == NULL ? nil : (__bridge BoxUSBHostDevice *)device->object;
}

static BoxUSBHostController *box_controller(box_usbhost_controller_t *controller) {
	return controller == NULL ? nil : (__bridge BoxUSBHostController *)controller->object;
}

static IOUSBHostCIDeviceStateMachine *box_device_sm(box_usbhost_device_sm_t *device) {
	return device == NULL ? nil : (__bridge IOUSBHostCIDeviceStateMachine *)device->object;
}

static IOUSBHostCIEndpointStateMachine *box_endpoint_sm(box_usbhost_endpoint_sm_t *endpoint) {
	return endpoint == NULL ? nil : (__bridge IOUSBHostCIEndpointStateMachine *)endpoint->object;
}

static void box_set_error_string(char **error_out, NSString *message) {
	if (error_out == NULL || *error_out != NULL) {
		return;
	}
	const char *utf8 = message.UTF8String;
	*error_out = strdup(utf8 != NULL ? utf8 : "unknown error");
}

static void box_set_error_from_nserror(char **error_out, NSString *prefix, NSError *error) {
	if (error == nil) {
		box_set_error_string(error_out, prefix);
		return;
	}
	NSString *description = error.localizedDescription ?: error.description;
	box_set_error_string(error_out, [NSString stringWithFormat:@"%@: %@", prefix, description]);
}

void box_usbhost_free_error(char *error) {
	free(error);
}

static uint32_t box_number_property(io_service_t service, NSString *key) {
	CFTypeRef value = IORegistryEntryCreateCFProperty(service, (__bridge CFStringRef)key, kCFAllocatorDefault, 0);
	if (value == NULL) {
		return 0;
	}
	uint32_t result = 0;
	if (CFGetTypeID(value) == CFNumberGetTypeID()) {
		CFNumberGetValue((CFNumberRef)value, kCFNumberSInt32Type, &result);
	}
	CFRelease(value);
	return result;
}

static void box_string_property(io_service_t service, NSString *key, char *dst, size_t dst_len) {
	if (dst_len == 0) {
		return;
	}
	dst[0] = 0;
	CFTypeRef value = IORegistryEntryCreateCFProperty(service, (__bridge CFStringRef)key, kCFAllocatorDefault, 0);
	if (value == NULL) {
		return;
	}
	if (CFGetTypeID(value) == CFStringGetTypeID()) {
		CFStringGetCString((CFStringRef)value, dst, dst_len, kCFStringEncodingUTF8);
	}
	CFRelease(value);
}

static uint32_t box_usbip_speed(uint32_t apple_speed) {
	switch (apple_speed) {
	case kIOUSBHostConnectionSpeedLow:
		return 1;
	case kIOUSBHostConnectionSpeedFull:
		return 2;
	case kIOUSBHostConnectionSpeedHigh:
		return 3;
	case kIOUSBHostConnectionSpeedSuper:
		return 5;
	case kIOUSBHostConnectionSpeedSuperPlus:
	case kIOUSBHostConnectionSpeedSuperPlusBy2:
		return 6;
	default:
		return 0;
	}
}

static IOUSBHostCIDeviceSpeed box_ci_speed(uint32_t usbip_speed) {
	switch (usbip_speed) {
	case 1:
		return IOUSBHostCIDeviceSpeedLow;
	case 2:
		return IOUSBHostCIDeviceSpeedFull;
	case 3:
		return IOUSBHostCIDeviceSpeedHigh;
	case 5:
		return IOUSBHostCIDeviceSpeedSuper;
	case 6:
		return IOUSBHostCIDeviceSpeedSuperPlus;
	default:
		return IOUSBHostCIDeviceSpeedFull;
	}
}

static void box_fill_busid(box_usbhost_device_info_t *info) {
	snprintf(info->busid, sizeof(info->busid), "mac-%08x", info->location_id);
}

static void box_fill_path(io_service_t service, box_usbhost_device_info_t *info) {
	io_string_t path;
	kern_return_t kr = IORegistryEntryGetPath(service, kIOServicePlane, path);
	if (kr == KERN_SUCCESS) {
		snprintf(info->path, sizeof(info->path), "IOService:%s", path);
	}
}

static void box_fill_registry_info(io_service_t service, box_usbhost_device_info_t *info) {
	memset(info, 0, sizeof(*info));
	uint64_t registry_id = 0;
	IORegistryEntryGetRegistryEntryID(service, &registry_id);
	info->registry_id = registry_id;
	info->location_id = box_number_property(service, IOUSBHostPropertyKeyLocationID);
	info->busnum = (info->location_id >> 24) & 0xff;
	if (info->busnum == 0) {
		info->busnum = 1;
	}
	info->devnum = (uint32_t)(registry_id & 0xffff);
	info->speed = box_usbip_speed(box_number_property(service, IOUSBHostMatchingPropertyKeySpeed));
	info->vendor_id = (uint16_t)box_number_property(service, IOUSBHostMatchingPropertyKeyVendorID);
	info->product_id = (uint16_t)box_number_property(service, IOUSBHostMatchingPropertyKeyProductID);
	info->bcd_device = (uint16_t)box_number_property(service, IOUSBHostMatchingPropertyKeyDeviceReleaseNumber);
	info->class_id = (uint8_t)box_number_property(service, IOUSBHostMatchingPropertyKeyDeviceClass);
	info->subclass_id = (uint8_t)box_number_property(service, IOUSBHostMatchingPropertyKeyDeviceSubClass);
	info->protocol_id = (uint8_t)box_number_property(service, IOUSBHostMatchingPropertyKeyDeviceProtocol);
	info->configuration_value = (uint8_t)box_number_property(service, IOUSBHostDevicePropertyKeyCurrentConfiguration);
	box_string_property(service, IOUSBHostDevicePropertyKeySerialNumberString, info->serial, sizeof(info->serial));
	box_fill_busid(info);
	box_fill_path(service, info);
}

static void box_fill_interface_info_from_descriptor(box_usbhost_device_info_t *info, const IOUSBInterfaceDescriptor *descriptor) {
	if (descriptor == NULL || info->interface_count >= BOX_USBHOST_MAX_INTERFACES) {
		return;
	}
	box_usbhost_interface_info_t *iface = &info->interfaces[info->interface_count++];
	iface->class_id = descriptor->bInterfaceClass;
	iface->subclass_id = descriptor->bInterfaceSubClass;
	iface->protocol_id = descriptor->bInterfaceProtocol;
}

static void box_fill_info_from_device(BoxUSBHostDevice *box, box_usbhost_device_info_t *info) {
	if (box.device == nil || info == NULL) {
		return;
	}
	NSError *error = nil;
	NSUInteger descriptor_length = kIOUSBDescriptorSizeDevice;
	const IOUSBDeviceDescriptor *device_descriptor = (const IOUSBDeviceDescriptor *)[box.device descriptorWithType:kIOUSBDescriptorTypeDevice
	                                                                                                        length:&descriptor_length
	                                                                                                         index:0
	                                                                                                    languageID:0
	                                                                                                         error:&error];
	if (device_descriptor != NULL) {
		info->vendor_id = USBToHost16(device_descriptor->idVendor);
		info->product_id = USBToHost16(device_descriptor->idProduct);
		info->bcd_device = USBToHost16(device_descriptor->bcdDevice);
		info->class_id = device_descriptor->bDeviceClass;
		info->subclass_id = device_descriptor->bDeviceSubClass;
		info->protocol_id = device_descriptor->bDeviceProtocol;
		info->num_configurations = device_descriptor->bNumConfigurations;
	}
	info->devnum = (uint32_t)box.device.deviceAddress;
	info->interface_count = 0;
	for (IOUSBHostInterface *interface in box.interfaces) {
		box_fill_interface_info_from_descriptor(info, interface.interfaceDescriptor);
	}
}

static void box_load_interfaces(BoxUSBHostDevice *box) {
	box.interfaces = [NSMutableArray array];
	io_iterator_t iterator = IO_OBJECT_NULL;
	kern_return_t kr = IORegistryEntryGetChildIterator(box.device.ioService, kIOServicePlane, &iterator);
	if (kr != KERN_SUCCESS) {
		return;
	}
	io_service_t child = IO_OBJECT_NULL;
	while ((child = IOIteratorNext(iterator)) != IO_OBJECT_NULL) {
		if (!IOObjectConformsTo(child, "IOUSBHostInterface")) {
			IOObjectRelease(child);
			continue;
		}
		NSError *error = nil;
		IOUSBHostInterface *interface = [[IOUSBHostInterface alloc] initWithIOService:child
		                                                                      options:IOUSBHostObjectInitOptionsNone
		                                                                        queue:nil
		                                                                        error:&error
		                                                              interestHandler:nil];
		if (interface != nil) {
			[box.interfaces addObject:interface];
		}
		IOObjectRelease(child);
	}
	IOObjectRelease(iterator);
}

static IOUSBHostPipe *box_pipe_for_endpoint(BoxUSBHostDevice *box, uint8_t endpoint) {
	NSNumber *key = @(endpoint);
	IOUSBHostPipe *cached = box.pipes[key];
	if (cached != nil) {
		return cached;
	}
	for (IOUSBHostInterface *interface in box.interfaces) {
		NSError *error = nil;
		IOUSBHostPipe *pipe = [interface copyPipeWithAddress:endpoint error:&error];
		if (pipe != nil) {
			box.pipes[key] = pipe;
			return pipe;
		}
	}
	return nil;
}

static IOUSBHostInterface *box_interface_for_number(BoxUSBHostDevice *box, uint8_t interface_number) {
	for (IOUSBHostInterface *interface in box.interfaces) {
		const IOUSBInterfaceDescriptor *descriptor = interface.interfaceDescriptor;
		if (descriptor != NULL && descriptor->bInterfaceNumber == interface_number) {
			return interface;
		}
	}
	return nil;
}

bool box_usbhost_copy_devices(box_usbhost_device_list_t *out, char **error_out) {
	if (out == NULL) {
		box_set_error_string(error_out, @"IOUSBHost copy devices: missing output");
		return false;
	}
	memset(out, 0, sizeof(*out));
	@autoreleasepool {
		CFMutableDictionaryRef matching = [IOUSBHostDevice createMatchingDictionaryWithVendorID:nil
		                                                                              productID:nil
		                                                                              bcdDevice:nil
		                                                                            deviceClass:nil
		                                                                         deviceSubclass:nil
		                                                                         deviceProtocol:nil
		                                                                                  speed:nil
		                                                                         productIDArray:nil];
		io_iterator_t iterator = IO_OBJECT_NULL;
		kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, matching, &iterator);
		if (kr != KERN_SUCCESS) {
			box_set_error_string(error_out, [NSString stringWithFormat:@"IOServiceGetMatchingServices(IOUSBHostDevice): 0x%x", kr]);
			return false;
		}
		NSMutableData *data = [NSMutableData data];
		io_service_t service = IO_OBJECT_NULL;
		while ((service = IOIteratorNext(iterator)) != IO_OBJECT_NULL) {
			box_usbhost_device_info_t info;
			box_fill_registry_info(service, &info);
			[data appendBytes:&info length:sizeof(info)];
			IOObjectRelease(service);
		}
		IOObjectRelease(iterator);
		out->count = data.length / sizeof(box_usbhost_device_info_t);
		if (out->count > 0) {
			out->items = malloc(data.length);
			if (out->items == NULL) {
				box_set_error_string(error_out, @"IOUSBHost copy devices: allocate list");
				out->count = 0;
				return false;
			}
			memcpy(out->items, data.bytes, data.length);
		}
		return true;
	}
}

void box_usbhost_device_list_free(box_usbhost_device_list_t *list) {
	if (list == NULL) {
		return;
	}
	free(list->items);
	list->items = NULL;
	list->count = 0;
}

box_usbhost_device_t *box_usbhost_device_open(uint64_t registry_id, bool capture, box_usbhost_device_info_t *info_out, char **error_out) {
	@autoreleasepool {
		CFMutableDictionaryRef matching = IORegistryEntryIDMatching(registry_id);
		io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, matching);
		if (service == IO_OBJECT_NULL) {
			box_set_error_string(error_out, [NSString stringWithFormat:@"IORegistryEntryIDMatching(%llu): no IOUSBHostDevice", registry_id]);
			return NULL;
		}
		box_usbhost_device_info_t info;
		box_fill_registry_info(service, &info);
		NSError *error = nil;
		IOUSBHostObjectInitOptions options = capture ? IOUSBHostObjectInitOptionsDeviceCapture : IOUSBHostObjectInitOptionsNone;
		IOUSBHostDevice *host_device = [[IOUSBHostDevice alloc] initWithIOService:service
		                                                                   options:options
		                                                                     queue:nil
		                                                                     error:&error
		                                                           interestHandler:nil];
		IOObjectRelease(service);
		if (host_device == nil) {
			box_set_error_from_nserror(error_out, @"IOUSBHostDevice initWithIOService", error);
			return NULL;
		}
		BoxUSBHostDevice *box = [[BoxUSBHostDevice alloc] init];
		box.device = host_device;
		box.pipes = [NSMutableDictionary dictionary];
		box_load_interfaces(box);
		box_fill_info_from_device(box, &info);
		if (info_out != NULL) {
			*info_out = info;
		}
		box_usbhost_device_t *handle = calloc(1, sizeof(*handle));
		handle->object = (void *)CFBridgingRetain(box);
		return handle;
	}
}

void box_usbhost_device_close(box_usbhost_device_t *device) {
	BoxUSBHostDevice *box = box_device(device);
	if (box != nil) {
		[box.device destroy];
		CFBridgingRelease(device->object);
	}
	free(device);
}

bool box_usbhost_device_control(box_usbhost_device_t *device, const uint8_t setup[8], uint8_t *data, size_t data_len, size_t *actual_out, int32_t *status_out, char **error_out) {
	BoxUSBHostDevice *box = box_device(device);
	if (box == nil || setup == NULL) {
		box_set_error_string(error_out, @"IOUSBHostDevice control: invalid handle");
		return false;
	}
	@autoreleasepool {
		IOUSBDeviceRequest request;
		request.bmRequestType = setup[0];
		request.bRequest = setup[1];
		request.wValue = setup[2] | ((uint16_t)setup[3] << 8);
		request.wIndex = setup[4] | ((uint16_t)setup[5] << 8);
		request.wLength = setup[6] | ((uint16_t)setup[7] << 8);
		NSMutableData *payload = nil;
		if (data_len > 0) {
			payload = [NSMutableData dataWithLength:data_len];
			if ((request.bmRequestType & kIOUSBDeviceRequestDirectionMask) == kIOUSBDeviceRequestDirectionOut && data != NULL) {
				memcpy(payload.mutableBytes, data, data_len);
			}
		}
		NSError *error = nil;
		NSUInteger actual = 0;
		BOOL ok = NO;
		if (request.bmRequestType == 0 && request.bRequest == kIOUSBDeviceRequestSetConfiguration && request.wIndex == 0 && request.wLength == 0) {
			ok = [box.device configureWithValue:request.wValue matchInterfaces:YES error:&error];
			if (ok) {
				box.pipes = [NSMutableDictionary dictionary];
				box_load_interfaces(box);
			}
		} else if (request.bmRequestType == kIOUSBDeviceRequestRecipientInterface && request.bRequest == kIOUSBDeviceRequestSetInterface && request.wLength == 0) {
			IOUSBHostInterface *interface = box_interface_for_number(box, request.wIndex & 0xff);
			if (interface == nil) {
				error = [NSError errorWithDomain:NSMachErrorDomain code:kIOReturnNotFound userInfo:nil];
			} else {
				ok = [interface selectAlternateSetting:request.wValue error:&error];
				if (ok) {
					box.pipes = [NSMutableDictionary dictionary];
					box_load_interfaces(box);
				}
			}
		} else {
			ok = [box.device sendDeviceRequest:request data:payload bytesTransferred:&actual completionTimeout:IOUSBHostDefaultControlCompletionTimeout error:&error];
		}
		if (actual_out != NULL) {
			*actual_out = actual;
		}
		if (status_out != NULL) {
			*status_out = ok ? 0 : (int32_t)(error != nil ? error.code : kIOReturnError);
		}
		if (ok && data_len > 0 && data != NULL && (request.bmRequestType & kIOUSBDeviceRequestDirectionMask) == kIOUSBDeviceRequestDirectionIn) {
			memcpy(data, payload.bytes, actual <= data_len ? actual : data_len);
		}
		return true;
	}
}

bool box_usbhost_device_io(box_usbhost_device_t *device, uint8_t endpoint, uint8_t *data, size_t data_len, size_t *actual_out, int32_t *status_out, char **error_out) {
	BoxUSBHostDevice *box = box_device(device);
	if (box == nil) {
		box_set_error_string(error_out, @"IOUSBHostPipe IO: invalid handle");
		return false;
	}
	IOUSBHostPipe *pipe = box_pipe_for_endpoint(box, endpoint);
	if (pipe == nil) {
		box_set_error_string(error_out, [NSString stringWithFormat:@"IOUSBHostInterface copyPipeWithAddress(0x%02x)", endpoint]);
		return false;
	}
	@autoreleasepool {
		NSMutableData *payload = data_len > 0 ? [NSMutableData dataWithLength:data_len] : nil;
		if ((endpoint & kIOUSBEndpointDescriptorDirection) == kIOUSBEndpointDescriptorDirectionOut && data_len > 0 && data != NULL) {
			memcpy(payload.mutableBytes, data, data_len);
		}
		NSError *error = nil;
		NSUInteger actual = 0;
		BOOL ok = [pipe sendIORequestWithData:payload bytesTransferred:&actual completionTimeout:0 error:&error];
		if (actual_out != NULL) {
			*actual_out = actual;
		}
		if (status_out != NULL) {
			*status_out = ok ? 0 : (int32_t)(error != nil ? error.code : kIOReturnError);
		}
		if (ok && (endpoint & kIOUSBEndpointDescriptorDirection) == kIOUSBEndpointDescriptorDirectionIn && data_len > 0 && data != NULL) {
			memcpy(data, payload.bytes, actual <= data_len ? actual : data_len);
		}
		return true;
	}
}

bool box_usbhost_device_iso(box_usbhost_device_t *device, uint8_t endpoint, uint8_t *data, size_t data_len, int32_t start_frame, box_usbhost_iso_packet_t *packets, size_t packet_count, size_t *actual_out, int32_t *status_out, char **error_out) {
	BoxUSBHostDevice *box = box_device(device);
	if (box == nil) {
		box_set_error_string(error_out, @"IOUSBHostPipe isochronous IO: invalid handle");
		return false;
	}
	IOUSBHostPipe *pipe = box_pipe_for_endpoint(box, endpoint);
	if (pipe == nil) {
		box_set_error_string(error_out, [NSString stringWithFormat:@"IOUSBHostInterface copyPipeWithAddress(0x%02x)", endpoint]);
		return false;
	}
	@autoreleasepool {
		NSMutableData *payload = data_len > 0 ? [NSMutableData dataWithLength:data_len] : nil;
		if ((endpoint & kIOUSBEndpointDescriptorDirection) == kIOUSBEndpointDescriptorDirectionOut && data_len > 0 && data != NULL) {
			memcpy(payload.mutableBytes, data, data_len);
		}
		IOUSBHostIsochronousTransaction *transactions = calloc(packet_count, sizeof(*transactions));
		if (transactions == NULL && packet_count > 0) {
			box_set_error_string(error_out, @"IOUSBHostPipe isochronous IO: allocate transactions");
			return false;
		}
		for (size_t i = 0; i < packet_count; i++) {
			transactions[i].requestCount = packets[i].length > 0 ? (uint32_t)packets[i].length : 0;
			transactions[i].offset = packets[i].offset > 0 ? (uint32_t)packets[i].offset : 0;
		}
		NSError *error = nil;
		BOOL ok = [pipe sendIORequestWithData:payload
		                      transactionList:transactions
		                 transactionListCount:packet_count
		                     firstFrameNumber:start_frame > 0 ? (uint64_t)start_frame : 0
		                              options:IOUSBHostIsochronousTransferOptionsNone
		                                error:&error];
		size_t actual = 0;
		for (size_t i = 0; i < packet_count; i++) {
			packets[i].actual_length = (int32_t)transactions[i].completeCount;
			packets[i].status = ok ? 0 : (int32_t)transactions[i].status;
			actual += transactions[i].completeCount;
		}
		free(transactions);
		if (actual_out != NULL) {
			*actual_out = actual;
		}
		if (status_out != NULL) {
			*status_out = ok ? 0 : (int32_t)(error != nil ? error.code : kIOReturnError);
		}
		if (ok && (endpoint & kIOUSBEndpointDescriptorDirection) == kIOUSBEndpointDescriptorDirectionIn && data_len > 0 && data != NULL) {
			memcpy(data, payload.bytes, actual <= data_len ? actual : data_len);
		}
		return true;
	}
}

bool box_usbhost_device_abort_endpoint(box_usbhost_device_t *device, uint8_t endpoint, char **error_out) {
	BoxUSBHostDevice *box = box_device(device);
	IOUSBHostPipe *pipe = box_pipe_for_endpoint(box, endpoint);
	if (pipe == nil) {
		return true;
	}
	NSError *error = nil;
	BOOL ok = [pipe abortWithOption:IOUSBHostAbortOptionAsynchronous error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostPipe abortWithOption", error);
	}
	return ok;
}

box_usbhost_controller_t *box_usbhost_controller_create(uintptr_t ref, uint8_t port_count, uint32_t speed, char **error_out) {
	@autoreleasepool {
		if (port_count == 0 || port_count > 15) {
			box_set_error_string(error_out, @"IOUSBHostControllerInterface: invalid port count");
			return NULL;
		}
		NSMutableData *capabilities = [NSMutableData data];
		IOUSBHostCIMessage controller_capabilities = {
			.control = (IOUSBHostCIMessageTypeControllerCapabilities << IOUSBHostCIMessageControlTypePhase)
				| IOUSBHostCIMessageControlNoResponse
				| IOUSBHostCIMessageControlValid
				| (port_count << IOUSBHostCICapabilitiesMessageControlPortCountPhase),
			.data0 = (1 << IOUSBHostCICapabilitiesMessageData0CommandTimeoutThresholdPhase)
				| (2 << IOUSBHostCICapabilitiesMessageData0ConnectionLatencyPhase),
			.data1 = 0,
		};
		[capabilities appendBytes:&controller_capabilities length:sizeof(controller_capabilities)];
		for (uint8_t port = 1; port <= port_count; port++) {
			IOUSBHostCIMessage port_capabilities = {
				.control = (IOUSBHostCIMessageTypePortCapabilities << IOUSBHostCIMessageControlTypePhase)
					| IOUSBHostCIMessageControlNoResponse
					| IOUSBHostCIMessageControlValid
					| (port << IOUSBHostCIPortCapabilitiesMessageControlPortNumberPhase),
				.data0 = (125 << IOUSBHostCIPortCapabilitiesMessageData0MaxPowerPhase),
				.data1 = 0,
			};
			[capabilities appendBytes:&port_capabilities length:sizeof(port_capabilities)];
		}
		IOUSBHostControllerInterfaceCommandHandler command_handler = ^(IOUSBHostControllerInterface *controller, IOUSBHostCIMessage command) {
			box_usbip_darwin_controller_command(ref, command);
		};
		IOUSBHostControllerInterfaceDoorbellHandler doorbell_handler = ^(IOUSBHostControllerInterface *controller, IOUSBHostCIDoorbell *doorbells, uint32_t count) {
			for (uint32_t i = 0; i < count; i++) {
				box_usbip_darwin_controller_doorbell(ref, doorbells[i]);
			}
		};
		NSError *error = nil;
		IOUSBHostControllerInterface *controller = [[IOUSBHostControllerInterface alloc] initWithCapabilities:capabilities
		                                                                                                queue:nil
		                                                                                      interruptRateHz:0
		                                                                                                error:&error
		                                                                                       commandHandler:command_handler
		                                                                                      doorbellHandler:doorbell_handler
		                                                                                      interestHandler:NULL];
		if (controller == nil || error.code != KERN_SUCCESS) {
			box_set_error_from_nserror(error_out, @"IOUSBHostControllerInterface initWithCapabilities", error);
			return NULL;
		}
		BoxUSBHostController *box = [[BoxUSBHostController alloc] init];
		box.controller = controller;
		box.ref = ref;
		box_usbhost_controller_t *handle = calloc(1, sizeof(*handle));
		handle->object = (void *)CFBridgingRetain(box);
		return handle;
	}
}

void box_usbhost_controller_destroy(box_usbhost_controller_t *controller) {
	BoxUSBHostController *box = box_controller(controller);
	if (box != nil) {
		[box.controller destroy];
		CFBridgingRelease(controller->object);
	}
	free(controller);
}

bool box_usbhost_controller_respond(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, char **error_out) {
	BoxUSBHostController *box = box_controller(controller);
	if (box == nil || command == NULL) {
		box_set_error_string(error_out, @"IOUSBHostCIControllerStateMachine respond: invalid handle");
		return false;
	}
	NSError *error = nil;
	IOUSBHostCIControllerStateMachine *state_machine = box.controller.controllerStateMachine;
	if (![state_machine inspectCommand:command error:&error]) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIControllerStateMachine inspectCommand", error);
		return false;
	}
	BOOL ok = [state_machine respondToCommand:command status:(IOUSBHostCIMessageStatus)status error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIControllerStateMachine respondToCommand", error);
	}
	return ok;
}

bool box_usbhost_controller_respond_frame(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, uint64_t frame, uint64_t timestamp, char **error_out) {
	BoxUSBHostController *box = box_controller(controller);
	NSError *error = nil;
	IOUSBHostCIControllerStateMachine *state_machine = box.controller.controllerStateMachine;
	if (![state_machine inspectCommand:command error:&error]) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIControllerStateMachine inspectCommand", error);
		return false;
	}
	BOOL ok = [state_machine respondToCommand:command status:(IOUSBHostCIMessageStatus)status frame:frame timestamp:timestamp error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIControllerStateMachine respondToCommand(frame)", error);
	}
	return ok;
}

bool box_usbhost_port_respond(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, bool powered, bool connected, uint32_t speed, char **error_out) {
	BoxUSBHostController *box = box_controller(controller);
	if (box == nil || command == NULL) {
		box_set_error_string(error_out, @"IOUSBHostCIPortStateMachine respond: invalid handle");
		return false;
	}
	NSError *error = nil;
	IOUSBHostCIPortStateMachine *port = [box.controller getPortStateMachineForCommand:command error:&error];
	if (port == nil || ![port inspectCommand:command error:&error]) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIPortStateMachine inspectCommand", error);
		return false;
	}
	port.powered = powered;
	if (powered) {
		port.connected = connected;
		[port updateLinkState:connected ? IOUSBHostCILinkStateU0 : IOUSBHostCILinkStateRxDetect speed:box_ci_speed(speed) inhibitLinkStateChange:NO error:&error];
	}
	BOOL ok = [port respondToCommand:command status:(IOUSBHostCIMessageStatus)status error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIPortStateMachine respondToCommand", error);
	}
	return ok;
}

box_usbhost_device_sm_t *box_usbhost_device_sm_create(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, char **error_out) {
	BoxUSBHostController *box = box_controller(controller);
	NSError *error = nil;
	IOUSBHostCIDeviceStateMachine *device = [[IOUSBHostCIDeviceStateMachine alloc] initWithInterface:box.controller command:command error:&error];
	if (device == nil) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIDeviceStateMachine initWithInterface", error);
		return NULL;
	}
	box_usbhost_device_sm_t *handle = calloc(1, sizeof(*handle));
	handle->object = (void *)CFBridgingRetain(device);
	return handle;
}

void box_usbhost_device_sm_destroy(box_usbhost_device_sm_t *device) {
	if (device != NULL && device->object != NULL) {
		CFBridgingRelease(device->object);
	}
	free(device);
}

bool box_usbhost_device_sm_respond(box_usbhost_device_sm_t *device, const IOUSBHostCIMessage *command, int status, char **error_out) {
	IOUSBHostCIDeviceStateMachine *state_machine = box_device_sm(device);
	NSError *error = nil;
	if (![state_machine inspectCommand:command error:&error]) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIDeviceStateMachine inspectCommand", error);
		return false;
	}
	BOOL ok = [state_machine respondToCommand:command status:(IOUSBHostCIMessageStatus)status error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIDeviceStateMachine respondToCommand", error);
	}
	return ok;
}

bool box_usbhost_device_sm_respond_create(box_usbhost_device_sm_t *device, const IOUSBHostCIMessage *command, int status, uint8_t device_address, char **error_out) {
	IOUSBHostCIDeviceStateMachine *state_machine = box_device_sm(device);
	NSError *error = nil;
	BOOL ok = [state_machine respondToCommand:command status:(IOUSBHostCIMessageStatus)status deviceAddress:device_address error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIDeviceStateMachine respondToCommand(create)", error);
	}
	return ok;
}

box_usbhost_endpoint_sm_t *box_usbhost_endpoint_sm_create(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, char **error_out) {
	BoxUSBHostController *box = box_controller(controller);
	NSError *error = nil;
	IOUSBHostCIEndpointStateMachine *endpoint = [[IOUSBHostCIEndpointStateMachine alloc] initWithInterface:box.controller command:command error:&error];
	if (endpoint == nil) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIEndpointStateMachine initWithInterface", error);
		return NULL;
	}
	box_usbhost_endpoint_sm_t *handle = calloc(1, sizeof(*handle));
	handle->object = (void *)CFBridgingRetain(endpoint);
	return handle;
}

void box_usbhost_endpoint_sm_destroy(box_usbhost_endpoint_sm_t *endpoint) {
	if (endpoint != NULL && endpoint->object != NULL) {
		CFBridgingRelease(endpoint->object);
	}
	free(endpoint);
}

bool box_usbhost_endpoint_sm_respond(box_usbhost_endpoint_sm_t *endpoint, const IOUSBHostCIMessage *command, int status, char **error_out) {
	IOUSBHostCIEndpointStateMachine *state_machine = box_endpoint_sm(endpoint);
	NSError *error = nil;
	if (![state_machine inspectCommand:command error:&error]) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIEndpointStateMachine inspectCommand", error);
		return false;
	}
	BOOL ok = [state_machine respondToCommand:command status:(IOUSBHostCIMessageStatus)status error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIEndpointStateMachine respondToCommand", error);
	}
	return ok;
}

bool box_usbhost_endpoint_sm_process_doorbell(box_usbhost_endpoint_sm_t *endpoint, uint32_t doorbell, char **error_out) {
	IOUSBHostCIEndpointStateMachine *state_machine = box_endpoint_sm(endpoint);
	NSError *error = nil;
	BOOL ok = [state_machine processDoorbell:(IOUSBHostCIDoorbell)doorbell error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIEndpointStateMachine processDoorbell", error);
	}
	return ok;
}

const IOUSBHostCIMessage *box_usbhost_endpoint_sm_current_transfer(box_usbhost_endpoint_sm_t *endpoint) {
	IOUSBHostCIEndpointStateMachine *state_machine = box_endpoint_sm(endpoint);
	return state_machine.currentTransferMessage;
}

bool box_usbhost_endpoint_sm_complete(box_usbhost_endpoint_sm_t *endpoint, const IOUSBHostCIMessage *message, int status, size_t transfer_length, char **error_out) {
	IOUSBHostCIEndpointStateMachine *state_machine = box_endpoint_sm(endpoint);
	NSError *error = nil;
	BOOL ok = [state_machine enqueueTransferCompletionForMessage:message status:(IOUSBHostCIMessageStatus)status transferLength:transfer_length error:&error];
	if (!ok) {
		box_set_error_from_nserror(error_out, @"IOUSBHostCIEndpointStateMachine enqueueTransferCompletion", error);
	}
	return ok;
}

uint8_t box_usbhost_ci_message_type(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)IOUSBHostCIMessageTypeFromMessage(message);
}

uint8_t box_usbhost_ci_message_status(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)IOUSBHostCIMessageStatusFromMessage(message);
}

bool box_usbhost_ci_message_valid(const IOUSBHostCIMessage *message) {
	return message != NULL && (message->control & IOUSBHostCIMessageControlValid) != 0;
}

bool box_usbhost_ci_message_no_response(const IOUSBHostCIMessage *message) {
	return message != NULL && (message->control & IOUSBHostCIMessageControlNoResponse) != 0;
}

uint8_t box_usbhost_ci_message_device_address(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)((message->data0 & IOUSBHostCICommandMessageData0DeviceAddress) >> IOUSBHostCICommandMessageData0DeviceAddressPhase);
}

uint8_t box_usbhost_ci_message_endpoint_address(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)((message->data0 & IOUSBHostCICommandMessageData0EndpointAddress) >> IOUSBHostCICommandMessageData0EndpointAddressPhase);
}

uint8_t box_usbhost_ci_command_root_port(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)((message->data0 & IOUSBHostCICommandMessageData0RootPort) >> IOUSBHostCICommandMessageData0RootPortPhase);
}

uint64_t box_usbhost_ci_setup_data(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : message->data1;
}

uint32_t box_usbhost_ci_normal_length(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint32_t)((message->data0 & IOUSBHostCINormalTransferData0Length) >> IOUSBHostCINormalTransferData0LengthPhase);
}

void *box_usbhost_ci_normal_buffer(const IOUSBHostCIMessage *message) {
	return message == NULL ? NULL : (void *)(uintptr_t)message->data1;
}

uint32_t box_usbhost_ci_iso_length(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint32_t)((message->data0 & IOUSBHostCIIsochronousTransferData0Length) >> IOUSBHostCIIsochronousTransferData0LengthPhase);
}

void *box_usbhost_ci_iso_buffer(const IOUSBHostCIMessage *message) {
	return message == NULL ? NULL : (void *)(uintptr_t)message->data1;
}

uint8_t box_usbhost_ci_iso_asap(const IOUSBHostCIMessage *message) {
	return message != NULL && (message->control & IOUSBHostCIIsochronousTransferControlASAP) != 0;
}

uint8_t box_usbhost_ci_iso_frame(const IOUSBHostCIMessage *message) {
	return message == NULL ? 0 : (uint8_t)((message->control & IOUSBHostCIIsochronousTransferControlFrameNumber) >> IOUSBHostCIIsochronousTransferControlFrameNumberPhase);
}

uint64_t box_usbhost_now(void) {
	return mach_absolute_time();
}
