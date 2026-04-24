#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include <IOUSBHost/IOUSBHost.h>

#define BOX_USBHOST_MAX_INTERFACES 255

typedef struct box_usbhost_interface_info {
	uint8_t class_id;
	uint8_t subclass_id;
	uint8_t protocol_id;
	uint8_t padding;
} box_usbhost_interface_info_t;

typedef struct box_usbhost_device_info {
	uint64_t registry_id;
	uint32_t location_id;
	uint32_t busnum;
	uint32_t devnum;
	uint32_t speed;
	uint16_t vendor_id;
	uint16_t product_id;
	uint16_t bcd_device;
	uint8_t class_id;
	uint8_t subclass_id;
	uint8_t protocol_id;
	uint8_t configuration_value;
	uint8_t num_configurations;
	uint8_t interface_count;
	char busid[32];
	char path[256];
	char serial[128];
	box_usbhost_interface_info_t interfaces[BOX_USBHOST_MAX_INTERFACES];
} box_usbhost_device_info_t;

typedef struct box_usbhost_device_list {
	size_t count;
	box_usbhost_device_info_t *items;
} box_usbhost_device_list_t;

typedef struct box_usbhost_iso_packet {
	int32_t offset;
	int32_t length;
	int32_t actual_length;
	int32_t status;
} box_usbhost_iso_packet_t;

typedef struct box_usbhost_device box_usbhost_device_t;
typedef struct box_usbhost_device_watcher box_usbhost_device_watcher_t;
typedef struct box_usbhost_controller box_usbhost_controller_t;
typedef struct box_usbhost_device_sm box_usbhost_device_sm_t;
typedef struct box_usbhost_endpoint_sm box_usbhost_endpoint_sm_t;

bool box_usbhost_copy_devices(box_usbhost_device_list_t *out, char **error_out);
void box_usbhost_device_list_free(box_usbhost_device_list_t *list);
box_usbhost_device_watcher_t *box_usbhost_device_watcher_create(uintptr_t ref, char **error_out);
void box_usbhost_device_watcher_destroy(box_usbhost_device_watcher_t *watcher);

box_usbhost_device_t *box_usbhost_device_open(uint64_t registry_id, bool capture, box_usbhost_device_info_t *info_out, char **error_out);
void box_usbhost_device_close(box_usbhost_device_t *device);
bool box_usbhost_device_control(box_usbhost_device_t *device, const uint8_t setup[8], uint8_t *data, size_t data_len, size_t *actual_out, int32_t *status_out, char **error_out);
bool box_usbhost_device_io(box_usbhost_device_t *device, uint8_t endpoint, uint8_t *data, size_t data_len, size_t *actual_out, int32_t *status_out, char **error_out);
bool box_usbhost_device_iso(box_usbhost_device_t *device, uint8_t endpoint, uint8_t *data, size_t data_len, int32_t start_frame, box_usbhost_iso_packet_t *packets, size_t packet_count, size_t *actual_out, int32_t *status_out, char **error_out);
bool box_usbhost_device_abort_endpoint(box_usbhost_device_t *device, uint8_t endpoint, char **error_out);

box_usbhost_controller_t *box_usbhost_controller_create(uintptr_t ref, uint8_t port_count, uint32_t speed, char **error_out);
void box_usbhost_controller_destroy(box_usbhost_controller_t *controller);
bool box_usbhost_controller_respond(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, char **error_out);
bool box_usbhost_controller_respond_frame(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, uint64_t frame, uint64_t timestamp, char **error_out);
bool box_usbhost_port_respond(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, int status, bool powered, bool connected, uint32_t speed, char **error_out);
box_usbhost_device_sm_t *box_usbhost_device_sm_create(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, char **error_out);
void box_usbhost_device_sm_destroy(box_usbhost_device_sm_t *device);
bool box_usbhost_device_sm_respond(box_usbhost_device_sm_t *device, const IOUSBHostCIMessage *command, int status, char **error_out);
bool box_usbhost_device_sm_respond_create(box_usbhost_device_sm_t *device, const IOUSBHostCIMessage *command, int status, uint8_t device_address, char **error_out);
box_usbhost_endpoint_sm_t *box_usbhost_endpoint_sm_create(box_usbhost_controller_t *controller, const IOUSBHostCIMessage *command, char **error_out);
void box_usbhost_endpoint_sm_destroy(box_usbhost_endpoint_sm_t *endpoint);
bool box_usbhost_endpoint_sm_respond(box_usbhost_endpoint_sm_t *endpoint, const IOUSBHostCIMessage *command, int status, char **error_out);
bool box_usbhost_endpoint_sm_process_doorbell(box_usbhost_endpoint_sm_t *endpoint, uint32_t doorbell, char **error_out);
const IOUSBHostCIMessage *box_usbhost_endpoint_sm_current_transfer(box_usbhost_endpoint_sm_t *endpoint);
bool box_usbhost_endpoint_sm_complete(box_usbhost_endpoint_sm_t *endpoint, const IOUSBHostCIMessage *message, int status, size_t transfer_length, char **error_out);

uint8_t box_usbhost_ci_message_type(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_message_status(const IOUSBHostCIMessage *message);
bool box_usbhost_ci_message_valid(const IOUSBHostCIMessage *message);
bool box_usbhost_ci_message_no_response(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_message_device_address(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_message_endpoint_address(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_command_root_port(const IOUSBHostCIMessage *message);
uint64_t box_usbhost_ci_setup_data(const IOUSBHostCIMessage *message);
uint32_t box_usbhost_ci_normal_length(const IOUSBHostCIMessage *message);
void *box_usbhost_ci_normal_buffer(const IOUSBHostCIMessage *message);
uint32_t box_usbhost_ci_iso_length(const IOUSBHostCIMessage *message);
void *box_usbhost_ci_iso_buffer(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_iso_asap(const IOUSBHostCIMessage *message);
uint8_t box_usbhost_ci_iso_frame(const IOUSBHostCIMessage *message);
uint64_t box_usbhost_now(void);

void box_usbhost_free_error(char *error);

extern void box_usbip_darwin_controller_command(uintptr_t ref, IOUSBHostCIMessage message);
extern void box_usbip_darwin_controller_doorbell(uintptr_t ref, uint32_t doorbell);
extern void box_usbip_darwin_usb_event(uintptr_t ref);
