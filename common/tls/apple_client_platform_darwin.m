#import "apple_client_platform_darwin.h"

#import <Foundation/Foundation.h>
#import <Network/Network.h>
#import <Security/Security.h>
#import <Security/SecProtocolMetadata.h>
#import <Security/SecProtocolOptions.h>
#import <Security/SecProtocolTypes.h>
#import <arpa/inet.h>
#import <dlfcn.h>
#import <dispatch/dispatch.h>
#import <stdatomic.h>
#import <stdlib.h>
#import <string.h>
#import <unistd.h>

typedef nw_connection_t _Nullable (*box_nw_connection_create_with_connected_socket_and_parameters_f)(int connected_socket, nw_parameters_t parameters);
typedef const char * _Nullable (*box_sec_protocol_metadata_string_accessor_f)(sec_protocol_metadata_t metadata);

typedef struct box_apple_tls_client {
	void *connection;
	void *queue;
	void *ready_semaphore;
	atomic_int ref_count;
	atomic_bool ready;
	atomic_bool ready_done;
	char *ready_error;
	box_apple_tls_state_t state;
} box_apple_tls_client_t;

static nw_connection_t box_apple_tls_connection(box_apple_tls_client_t *client) {
	if (client == NULL || client->connection == NULL) {
		return nil;
	}
	return (__bridge nw_connection_t)client->connection;
}

static dispatch_queue_t box_apple_tls_client_queue(box_apple_tls_client_t *client) {
	if (client == NULL || client->queue == NULL) {
		return nil;
	}
	return (__bridge dispatch_queue_t)client->queue;
}

static dispatch_semaphore_t box_apple_tls_ready_semaphore(box_apple_tls_client_t *client) {
	if (client == NULL || client->ready_semaphore == NULL) {
		return nil;
	}
	return (__bridge dispatch_semaphore_t)client->ready_semaphore;
}

static void box_apple_tls_state_reset(box_apple_tls_state_t *state) {
	if (state == NULL) {
		return;
	}
	free(state->alpn);
	free(state->server_name);
	free(state->peer_cert_chain);
	memset(state, 0, sizeof(box_apple_tls_state_t));
}

static void box_apple_tls_client_destroy(box_apple_tls_client_t *client) {
	free(client->ready_error);
	box_apple_tls_state_reset(&client->state);
	if (client->ready_semaphore != NULL) {
		CFBridgingRelease(client->ready_semaphore);
	}
	if (client->connection != NULL) {
		CFBridgingRelease(client->connection);
	}
	if (client->queue != NULL) {
		CFBridgingRelease(client->queue);
	}
	free(client);
}

static void box_apple_tls_client_release(box_apple_tls_client_t *client) {
	if (client == NULL) {
		return;
	}
	if (atomic_fetch_sub(&client->ref_count, 1) == 1) {
		box_apple_tls_client_destroy(client);
	}
}

static void box_set_error_string(char **error_out, NSString *message) {
	if (error_out == NULL || *error_out != NULL) {
		return;
	}
	const char *utf8 = [message UTF8String];
	*error_out = strdup(utf8 != NULL ? utf8 : "unknown error");
}

static void box_set_error_message(char **error_out, const char *message) {
	if (error_out == NULL || *error_out != NULL) {
		return;
	}
	*error_out = strdup(message != NULL ? message : "unknown error");
}

static void box_set_error_from_nw_error(char **error_out, nw_error_t error) {
	if (error == NULL) {
		box_set_error_message(error_out, "unknown network error");
		return;
	}
	CFErrorRef cfError = nw_error_copy_cf_error(error);
	if (cfError == NULL) {
		box_set_error_message(error_out, "unknown network error");
		return;
	}
	NSString *description = [(__bridge NSError *)cfError description];
	box_set_error_string(error_out, description);
	CFRelease(cfError);
}

static char *box_apple_tls_metadata_copy_negotiated_protocol(sec_protocol_metadata_t metadata) {
	static box_sec_protocol_metadata_string_accessor_f copy_fn;
	static box_sec_protocol_metadata_string_accessor_f get_fn;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		copy_fn = (box_sec_protocol_metadata_string_accessor_f)dlsym(RTLD_DEFAULT, "sec_protocol_metadata_copy_negotiated_protocol");
		get_fn = (box_sec_protocol_metadata_string_accessor_f)dlsym(RTLD_DEFAULT, "sec_protocol_metadata_get_negotiated_protocol");
	});
	if (copy_fn != NULL) {
		return (char *)copy_fn(metadata);
	}
	if (get_fn != NULL) {
		const char *protocol = get_fn(metadata);
		if (protocol != NULL) {
			return strdup(protocol);
		}
	}
	return NULL;
}

static char *box_apple_tls_metadata_copy_server_name(sec_protocol_metadata_t metadata) {
	static box_sec_protocol_metadata_string_accessor_f copy_fn;
	static box_sec_protocol_metadata_string_accessor_f get_fn;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		copy_fn = (box_sec_protocol_metadata_string_accessor_f)dlsym(RTLD_DEFAULT, "sec_protocol_metadata_copy_server_name");
		get_fn = (box_sec_protocol_metadata_string_accessor_f)dlsym(RTLD_DEFAULT, "sec_protocol_metadata_get_server_name");
	});
	if (copy_fn != NULL) {
		return (char *)copy_fn(metadata);
	}
	if (get_fn != NULL) {
		const char *server_name = get_fn(metadata);
		if (server_name != NULL) {
			return strdup(server_name);
		}
	}
	return NULL;
}

static NSArray<NSString *> *box_split_lines(const char *content, size_t content_len) {
	if (content == NULL || content_len == 0) {
		return @[];
	}
	NSString *string = [[NSString alloc] initWithBytes:content length:content_len encoding:NSUTF8StringEncoding];
	if (string == nil) {
		return @[];
	}
	NSMutableArray<NSString *> *lines = [NSMutableArray array];
	[string enumerateLinesUsingBlock:^(NSString *line, BOOL *stop) {
		if (line.length > 0) {
			[lines addObject:line];
		}
	}];
	return lines;
}

static NSArray *box_parse_certificates_from_pem(const char *pem, size_t pem_len) {
	if (pem == NULL || pem_len == 0) {
		return @[];
	}
	NSString *content = [[NSString alloc] initWithBytes:pem length:pem_len encoding:NSUTF8StringEncoding];
	if (content == nil) {
		return @[];
	}
	NSString *beginMarker = @"-----BEGIN CERTIFICATE-----";
	NSString *endMarker = @"-----END CERTIFICATE-----";
	NSMutableArray *certificates = [NSMutableArray array];
	NSUInteger searchFrom = 0;
	while (searchFrom < content.length) {
		NSRange beginRange = [content rangeOfString:beginMarker options:0 range:NSMakeRange(searchFrom, content.length - searchFrom)];
		if (beginRange.location == NSNotFound) {
			break;
		}
		NSUInteger bodyStart = beginRange.location + beginRange.length;
		NSRange endRange = [content rangeOfString:endMarker options:0 range:NSMakeRange(bodyStart, content.length - bodyStart)];
		if (endRange.location == NSNotFound) {
			break;
		}
		NSString *base64Section = [content substringWithRange:NSMakeRange(bodyStart, endRange.location - bodyStart)];
		NSArray<NSString *> *components = [base64Section componentsSeparatedByCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
		NSString *base64Content = [components componentsJoinedByString:@""];
		NSData *der = [[NSData alloc] initWithBase64EncodedString:base64Content options:0];
		if (der != nil) {
			SecCertificateRef certificate = SecCertificateCreateWithData(NULL, (__bridge CFDataRef)der);
			if (certificate != NULL) {
				[certificates addObject:(__bridge id)certificate];
				CFRelease(certificate);
			}
		}
		searchFrom = endRange.location + endRange.length;
	}
	return certificates;
}

static bool box_evaluate_trust(sec_trust_t trust, NSArray *anchors, bool anchor_only, NSDate *verify_date) {
	bool result = false;
	SecTrustRef trustRef = sec_trust_copy_ref(trust);
	if (trustRef == NULL) {
		return false;
	}
	if (verify_date != nil && SecTrustSetVerifyDate(trustRef, (__bridge CFDateRef)verify_date) != errSecSuccess) {
		CFRelease(trustRef);
		return false;
	}
	if (anchors.count > 0 || anchor_only) {
		CFMutableArrayRef anchorArray = CFArrayCreateMutable(NULL, 0, &kCFTypeArrayCallBacks);
		for (id certificate in anchors) {
			CFArrayAppendValue(anchorArray, (__bridge const void *)certificate);
		}
		SecTrustSetAnchorCertificates(trustRef, anchorArray);
		SecTrustSetAnchorCertificatesOnly(trustRef, anchor_only);
		CFRelease(anchorArray);
	}
	CFErrorRef error = NULL;
	result = SecTrustEvaluateWithError(trustRef, &error);
	if (error != NULL) {
		CFRelease(error);
	}
	CFRelease(trustRef);
	return result;
}

static nw_connection_t box_apple_tls_create_connection(int connected_socket, nw_parameters_t parameters) {
	static box_nw_connection_create_with_connected_socket_and_parameters_f create_fn;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		char name[] = "sretemarap_dna_tekcos_detcennoc_htiw_etaerc_noitcennoc_wn";
		for (size_t i = 0, j = sizeof(name) - 2; i < j; i++, j--) {
			char t = name[i];
			name[i] = name[j];
			name[j] = t;
		}
		create_fn = (box_nw_connection_create_with_connected_socket_and_parameters_f)dlsym(RTLD_DEFAULT, name);
	});
	if (create_fn == NULL) {
		return nil;
	}
	return create_fn(connected_socket, parameters);
}

static bool box_apple_tls_state_copy(const box_apple_tls_state_t *source, box_apple_tls_state_t *destination) {
	memset(destination, 0, sizeof(box_apple_tls_state_t));
	destination->version = source->version;
	destination->cipher_suite = source->cipher_suite;
	if (source->alpn != NULL) {
		destination->alpn = strdup(source->alpn);
		if (destination->alpn == NULL) {
			goto oom;
		}
	}
	if (source->server_name != NULL) {
		destination->server_name = strdup(source->server_name);
		if (destination->server_name == NULL) {
			goto oom;
		}
	}
	if (source->peer_cert_chain_len > 0) {
		destination->peer_cert_chain = malloc(source->peer_cert_chain_len);
		if (destination->peer_cert_chain == NULL) {
			goto oom;
		}
		memcpy(destination->peer_cert_chain, source->peer_cert_chain, source->peer_cert_chain_len);
		destination->peer_cert_chain_len = source->peer_cert_chain_len;
	}
	return true;

oom:
	box_apple_tls_state_reset(destination);
	return false;
}

static bool box_apple_tls_state_load(nw_connection_t connection, box_apple_tls_state_t *state, char **error_out) {
	box_apple_tls_state_reset(state);
	if (connection == nil) {
		box_set_error_message(error_out, "apple TLS: invalid client");
		return false;
	}

	nw_protocol_definition_t tls_definition = nw_protocol_copy_tls_definition();
	nw_protocol_metadata_t metadata = nw_connection_copy_protocol_metadata(connection, tls_definition);
	if (metadata == NULL || !nw_protocol_metadata_is_tls(metadata)) {
		box_set_error_message(error_out, "apple TLS: metadata unavailable");
		return false;
	}

	sec_protocol_metadata_t sec_metadata = nw_tls_copy_sec_protocol_metadata(metadata);
	if (sec_metadata == NULL) {
		box_set_error_message(error_out, "apple TLS: metadata unavailable");
		return false;
	}

	state->version = (uint16_t)sec_protocol_metadata_get_negotiated_tls_protocol_version(sec_metadata);
	state->cipher_suite = (uint16_t)sec_protocol_metadata_get_negotiated_tls_ciphersuite(sec_metadata);
	state->alpn = box_apple_tls_metadata_copy_negotiated_protocol(sec_metadata);
	state->server_name = box_apple_tls_metadata_copy_server_name(sec_metadata);

	NSMutableData *chain_data = [NSMutableData data];
	sec_protocol_metadata_access_peer_certificate_chain(sec_metadata, ^(sec_certificate_t certificate) {
		SecCertificateRef certificate_ref = sec_certificate_copy_ref(certificate);
		if (certificate_ref == NULL) {
			return;
		}
		CFDataRef certificate_data = SecCertificateCopyData(certificate_ref);
		CFRelease(certificate_ref);
		if (certificate_data == NULL) {
			return;
		}
		uint32_t certificate_len = (uint32_t)CFDataGetLength(certificate_data);
		uint32_t network_len = htonl(certificate_len);
		[chain_data appendBytes:&network_len length:sizeof(network_len)];
		[chain_data appendBytes:CFDataGetBytePtr(certificate_data) length:certificate_len];
		CFRelease(certificate_data);
	});
	if (chain_data.length > 0) {
		state->peer_cert_chain = malloc(chain_data.length);
		if (state->peer_cert_chain == NULL) {
			box_set_error_message(error_out, "apple TLS: out of memory");
			box_apple_tls_state_reset(state);
			return false;
		}
		memcpy(state->peer_cert_chain, chain_data.bytes, chain_data.length);
		state->peer_cert_chain_len = chain_data.length;
	}
	return true;
}

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
) {
	box_apple_tls_client_t *client = calloc(1, sizeof(box_apple_tls_client_t));
	if (client == NULL) {
		close(connected_socket);
		box_set_error_message(error_out, "apple TLS: out of memory");
		return NULL;
	}
	client->queue = (__bridge_retained void *)dispatch_queue_create("sing-box.apple-private-tls", DISPATCH_QUEUE_SERIAL);
	client->ready_semaphore = (__bridge_retained void *)dispatch_semaphore_create(0);
	atomic_init(&client->ref_count, 1);
	atomic_init(&client->ready, false);
	atomic_init(&client->ready_done, false);

	NSArray<NSString *> *alpnList = box_split_lines(alpn, alpn_len);
	NSArray *anchors = box_parse_certificates_from_pem(anchor_pem, anchor_pem_len);
	NSDate *verifyDate = nil;
	if (has_verify_time) {
		verifyDate = [NSDate dateWithTimeIntervalSince1970:(NSTimeInterval)verify_time_unix_millis / 1000.0];
	}
	nw_parameters_t parameters = nw_parameters_create_secure_tcp(^(nw_protocol_options_t tls_options) {
		sec_protocol_options_t sec_options = nw_tls_copy_sec_protocol_options(tls_options);
		if (min_version != 0) {
			sec_protocol_options_set_min_tls_protocol_version(sec_options, (tls_protocol_version_t)min_version);
		}
		if (max_version != 0) {
			sec_protocol_options_set_max_tls_protocol_version(sec_options, (tls_protocol_version_t)max_version);
		}
		if (server_name != NULL && server_name[0] != '\0') {
			sec_protocol_options_set_tls_server_name(sec_options, server_name);
		}
		for (NSString *protocol in alpnList) {
			sec_protocol_options_add_tls_application_protocol(sec_options, protocol.UTF8String);
		}
		sec_protocol_options_set_peer_authentication_required(sec_options, !insecure);
		if (insecure) {
			sec_protocol_options_set_verify_block(sec_options, ^(sec_protocol_metadata_t metadata, sec_trust_t trust, sec_protocol_verify_complete_t complete) {
				complete(true);
			}, box_apple_tls_client_queue(client));
		} else if (verifyDate != nil || anchors.count > 0 || anchor_only) {
			sec_protocol_options_set_verify_block(sec_options, ^(sec_protocol_metadata_t metadata, sec_trust_t trust, sec_protocol_verify_complete_t complete) {
				complete(box_evaluate_trust(trust, anchors, anchor_only, verifyDate));
			}, box_apple_tls_client_queue(client));
		}
	}, NW_PARAMETERS_DEFAULT_CONFIGURATION);

	nw_connection_t connection = box_apple_tls_create_connection(connected_socket, parameters);
	if (connection == NULL) {
		close(connected_socket);
		if (client->ready_semaphore != NULL) {
			CFBridgingRelease(client->ready_semaphore);
		}
		if (client->queue != NULL) {
			CFBridgingRelease(client->queue);
		}
		free(client);
		box_set_error_message(error_out, "apple TLS: failed to create connection");
		return NULL;
	}

	client->connection = (__bridge_retained void *)connection;
	atomic_fetch_add(&client->ref_count, 1);

	nw_connection_set_state_changed_handler(connection, ^(nw_connection_state_t state, nw_error_t error) {
		switch (state) {
		case nw_connection_state_ready:
			if (!atomic_load(&client->ready_done)) {
				atomic_store(&client->ready, box_apple_tls_state_load(connection, &client->state, &client->ready_error));
				atomic_store(&client->ready_done, true);
				dispatch_semaphore_signal(box_apple_tls_ready_semaphore(client));
			}
			break;
		case nw_connection_state_failed:
			if (!atomic_load(&client->ready_done)) {
				box_set_error_from_nw_error(&client->ready_error, error);
				atomic_store(&client->ready_done, true);
				dispatch_semaphore_signal(box_apple_tls_ready_semaphore(client));
			}
			break;
		case nw_connection_state_cancelled:
			if (!atomic_load(&client->ready_done)) {
				box_set_error_from_nw_error(&client->ready_error, error);
				atomic_store(&client->ready_done, true);
				dispatch_semaphore_signal(box_apple_tls_ready_semaphore(client));
			}
			box_apple_tls_client_release(client);
			break;
		default:
			break;
		}
	});
	nw_connection_set_queue(connection, box_apple_tls_client_queue(client));
	nw_connection_start(connection);
	return client;
}

int box_apple_tls_client_wait_ready(box_apple_tls_client_t *client, int timeout_msec, char **error_out) {
	dispatch_semaphore_t ready_semaphore = box_apple_tls_ready_semaphore(client);
	if (ready_semaphore == nil) {
		box_set_error_message(error_out, "apple TLS: invalid client");
		return 0;
	}
	if (!atomic_load(&client->ready_done)) {
		dispatch_time_t timeout = DISPATCH_TIME_FOREVER;
		if (timeout_msec >= 0) {
			timeout = dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeout_msec * NSEC_PER_MSEC);
		}
		long wait_result = dispatch_semaphore_wait(ready_semaphore, timeout);
		if (wait_result != 0) {
			return -2;
		}
	}
	if (atomic_load(&client->ready)) {
		return 1;
	}
	if (client->ready_error != NULL) {
		if (error_out != NULL) {
			*error_out = client->ready_error;
			client->ready_error = NULL;
		} else {
			free(client->ready_error);
			client->ready_error = NULL;
		}
	} else {
		box_set_error_message(error_out, "apple TLS: handshake failed");
	}
	return 0;
}

void box_apple_tls_client_cancel(box_apple_tls_client_t *client) {
	if (client == NULL) {
		return;
	}
	nw_connection_t connection = box_apple_tls_connection(client);
	if (connection != nil) {
		nw_connection_cancel(connection);
	}
}

void box_apple_tls_client_free(box_apple_tls_client_t *client) {
	if (client == NULL) {
		return;
	}
	nw_connection_t connection = box_apple_tls_connection(client);
	if (connection != nil) {
		nw_connection_cancel(connection);
	}
	box_apple_tls_client_release(client);
}

ssize_t box_apple_tls_client_read(box_apple_tls_client_t *client, void *buffer, size_t buffer_len, int timeout_msec, bool *eof_out, char **error_out) {
	nw_connection_t connection = box_apple_tls_connection(client);
	if (connection == nil) {
		box_set_error_message(error_out, "apple TLS: invalid client");
		return -1;
	}

	dispatch_semaphore_t read_semaphore = dispatch_semaphore_create(0);
	__block NSData *content_data = nil;
	__block bool read_eof = false;
	__block char *local_error = NULL;

	nw_connection_receive(connection, 1, (uint32_t)buffer_len, ^(dispatch_data_t content, nw_content_context_t context, bool is_complete, nw_error_t error) {
		if (content != NULL) {
			const void *mapped = NULL;
			size_t mapped_len = 0;
			dispatch_data_t mapped_data = dispatch_data_create_map(content, &mapped, &mapped_len);
			if (mapped != NULL && mapped_len > 0) {
				content_data = [NSData dataWithBytes:mapped length:mapped_len];
			}
			(void)mapped_data;
		}
		if (error != NULL && content_data.length == 0) {
			box_set_error_from_nw_error(&local_error, error);
		}
		if (is_complete && (context == NULL || nw_content_context_get_is_final(context))) {
			read_eof = true;
		}
		dispatch_semaphore_signal(read_semaphore);
	});

	dispatch_time_t wait_deadline = DISPATCH_TIME_FOREVER;
	if (timeout_msec >= 0) {
		wait_deadline = dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeout_msec * NSEC_PER_MSEC);
	}
	long wait_result = dispatch_semaphore_wait(read_semaphore, wait_deadline);
	if (wait_result != 0) {
		nw_connection_cancel(connection);
		dispatch_semaphore_wait(read_semaphore, DISPATCH_TIME_FOREVER);
		if (local_error != NULL) {
			free(local_error);
			local_error = NULL;
		}
		return -2;
	}
	if (local_error != NULL) {
		if (error_out != NULL) {
			*error_out = local_error;
		} else {
			free(local_error);
		}
		return -1;
	}
	if (eof_out != NULL) {
		*eof_out = read_eof;
	}
	if (content_data == nil || content_data.length == 0) {
		return 0;
	}
	memcpy(buffer, content_data.bytes, content_data.length);
	return (ssize_t)content_data.length;
}

ssize_t box_apple_tls_client_write(box_apple_tls_client_t *client, const void *buffer, size_t buffer_len, int timeout_msec, char **error_out) {
	nw_connection_t connection = box_apple_tls_connection(client);
	if (connection == nil) {
		box_set_error_message(error_out, "apple TLS: invalid client");
		return -1;
	}
	if (buffer_len == 0) {
		return 0;
	}

	void *content_copy = malloc(buffer_len);
	dispatch_queue_t queue = box_apple_tls_client_queue(client);
	if (content_copy == NULL) {
		free(content_copy);
		box_set_error_message(error_out, "apple TLS: out of memory");
		return -1;
	}
	if (queue == nil) {
		free(content_copy);
		box_set_error_message(error_out, "apple TLS: invalid client");
		return -1;
	}
	memcpy(content_copy, buffer, buffer_len);
	dispatch_data_t content = dispatch_data_create(content_copy, buffer_len, queue, ^{
		free(content_copy);
	});

	dispatch_semaphore_t write_semaphore = dispatch_semaphore_create(0);
	__block char *local_error = NULL;

	nw_connection_send(connection, content, NW_CONNECTION_DEFAULT_STREAM_CONTEXT, false, ^(nw_error_t error) {
		if (error != NULL) {
			box_set_error_from_nw_error(&local_error, error);
		}
		dispatch_semaphore_signal(write_semaphore);
	});

	dispatch_time_t wait_deadline = DISPATCH_TIME_FOREVER;
	if (timeout_msec >= 0) {
		wait_deadline = dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeout_msec * NSEC_PER_MSEC);
	}
	long wait_result = dispatch_semaphore_wait(write_semaphore, wait_deadline);
	if (wait_result != 0) {
		nw_connection_cancel(connection);
		dispatch_semaphore_wait(write_semaphore, DISPATCH_TIME_FOREVER);
		if (local_error != NULL) {
			free(local_error);
			local_error = NULL;
		}
		return -2;
	}
	if (local_error != NULL) {
		if (error_out != NULL) {
			*error_out = local_error;
		} else {
			free(local_error);
		}
		return -1;
	}
	return (ssize_t)buffer_len;
}

bool box_apple_tls_client_copy_state(box_apple_tls_client_t *client, box_apple_tls_state_t *state, char **error_out) {
	dispatch_queue_t queue = box_apple_tls_client_queue(client);
	if (queue == nil || state == NULL) {
		box_set_error_message(error_out, "apple TLS: invalid client");
		return false;
	}
	memset(state, 0, sizeof(box_apple_tls_state_t));
	__block bool copied = false;
	__block char *local_error = NULL;
	dispatch_sync(queue, ^{
		if (!atomic_load(&client->ready)) {
			box_set_error_message(&local_error, "apple TLS: metadata unavailable");
			return;
		}
		if (!box_apple_tls_state_copy(&client->state, state)) {
			box_set_error_message(&local_error, "apple TLS: out of memory");
			return;
		}
		copied = true;
	});
	if (copied) {
		return true;
	}
	if (local_error != NULL) {
		if (error_out != NULL) {
			*error_out = local_error;
		} else {
			free(local_error);
		}
	}
	box_apple_tls_state_reset(state);
	return false;
}

void box_apple_tls_state_free(box_apple_tls_state_t *state) {
	box_apple_tls_state_reset(state);
}
