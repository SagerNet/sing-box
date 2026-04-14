#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

typedef struct box_apple_http_session box_apple_http_session_t;
typedef struct box_apple_http_task box_apple_http_task_t;

typedef struct box_apple_http_session_config {
	const char *proxy_host;
	int proxy_port;
	const char *proxy_username;
	const char *proxy_password;
	uint16_t min_tls_version;
	uint16_t max_tls_version;
	bool insecure;
	const char *anchor_pem;
	size_t anchor_pem_len;
	bool anchor_only;
	const uint8_t *pinned_public_key_sha256;
	size_t pinned_public_key_sha256_len;
} box_apple_http_session_config_t;

typedef struct box_apple_http_request {
	const char *method;
	const char *url;
	const char **header_keys;
	const char **header_values;
	size_t header_count;
	const uint8_t *body;
	size_t body_len;
	bool has_verify_time;
	int64_t verify_time_unix_millis;
} box_apple_http_request_t;

typedef struct box_apple_http_response {
	int status_code;
	char **header_keys;
	char **header_values;
	size_t header_count;
	uint8_t *body;
	size_t body_len;
	char *error;
} box_apple_http_response_t;

box_apple_http_session_t *box_apple_http_session_create(
	const box_apple_http_session_config_t *config,
	char **error_out
);
void box_apple_http_session_retire(box_apple_http_session_t *session);
void box_apple_http_session_close(box_apple_http_session_t *session);

box_apple_http_task_t *box_apple_http_session_send_async(
	box_apple_http_session_t *session,
	const box_apple_http_request_t *request,
	char **error_out
);
box_apple_http_response_t *box_apple_http_task_wait(
	box_apple_http_task_t *task,
	char **error_out
);
void box_apple_http_task_cancel(box_apple_http_task_t *task);
void box_apple_http_task_close(box_apple_http_task_t *task);

void box_apple_http_response_free(box_apple_http_response_t *response);

char *box_apple_http_verify_public_key_sha256(
	uint8_t *known_hash_values,
	size_t known_hash_values_len,
	uint8_t *leaf_cert,
	size_t leaf_cert_len
);
