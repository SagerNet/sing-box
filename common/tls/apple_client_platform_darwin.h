#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <unistd.h>

typedef struct box_apple_tls_client box_apple_tls_client_t;

typedef struct box_apple_tls_state {
	uint16_t version;
	uint16_t cipher_suite;
	char *alpn;
	char *server_name;
	uint8_t *peer_cert_chain;
	size_t peer_cert_chain_len;
} box_apple_tls_state_t;

box_apple_tls_client_t *box_apple_tls_client_create(
	int connected_socket,
	const char *server_name,
	const char *alpn,
	size_t alpn_len,
	uint16_t min_version,
	uint16_t max_version,
	bool insecure,
	const char *anchor_pem,
	size_t anchor_pem_len,
	bool anchor_only,
	bool has_verify_time,
	int64_t verify_time_unix_millis,
	char **error_out
);

int box_apple_tls_client_wait_ready(box_apple_tls_client_t *client, int timeout_msec, char **error_out);
void box_apple_tls_client_cancel(box_apple_tls_client_t *client);
void box_apple_tls_client_free(box_apple_tls_client_t *client);
ssize_t box_apple_tls_client_read(box_apple_tls_client_t *client, void *buffer, size_t buffer_len, int timeout_msec, bool *eof_out, char **error_out);
ssize_t box_apple_tls_client_write(box_apple_tls_client_t *client, const void *buffer, size_t buffer_len, int timeout_msec, char **error_out);
bool box_apple_tls_client_copy_state(box_apple_tls_client_t *client, box_apple_tls_state_t *state, char **error_out);
void box_apple_tls_state_free(box_apple_tls_state_t *state);
