#import "apple_transport_darwin.h"

#import <CoreFoundation/CFStream.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>
#import <dispatch/dispatch.h>
#import <stdlib.h>
#import <string.h>

typedef struct box_apple_http_session {
	void *handle;
} box_apple_http_session_t;

typedef struct box_apple_http_task {
	void *task;
	void *done_semaphore;
	box_apple_http_response_t *response;
	char *error;
} box_apple_http_task_t;

static NSString *const box_apple_http_verify_time_key = @"sing-box.verify-time";

static void box_set_error_string(char **error_out, NSString *message) {
	if (error_out == NULL || *error_out != NULL) {
		return;
	}
	const char *utf8 = [message UTF8String];
	*error_out = strdup(utf8 != NULL ? utf8 : "unknown error");
}

static void box_set_error_from_nserror(char **error_out, NSError *error) {
	if (error == nil) {
		box_set_error_string(error_out, @"unknown error");
		return;
	}
	box_set_error_string(error_out, error.localizedDescription ?: error.description);
}

static bool box_evaluate_trust(SecTrustRef trustRef, NSArray *anchors, bool anchor_only, NSDate *verifyDate) {
	if (trustRef == NULL) {
		return false;
	}
	if (verifyDate != nil && SecTrustSetVerifyDate(trustRef, (__bridge CFDateRef)verifyDate) != errSecSuccess) {
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
	bool result = SecTrustEvaluateWithError(trustRef, &error);
	if (error != NULL) {
		CFRelease(error);
	}
	return result;
}

static NSDate *box_apple_http_verify_date_for_request(NSURLRequest *request) {
	if (request == nil) {
		return nil;
	}
	id value = [NSURLProtocol propertyForKey:box_apple_http_verify_time_key inRequest:request];
	if (![value isKindOfClass:[NSNumber class]]) {
		return nil;
	}
	return [NSDate dateWithTimeIntervalSince1970:[(NSNumber *)value longLongValue] / 1000.0];
}

static box_apple_http_response_t *box_create_response(NSHTTPURLResponse *httpResponse, NSData *data) {
	box_apple_http_response_t *response = calloc(1, sizeof(box_apple_http_response_t));
	response->status_code = (int)httpResponse.statusCode;
	NSDictionary *headers = httpResponse.allHeaderFields;
	response->header_count = headers.count;
	if (response->header_count > 0) {
		response->header_keys = calloc(response->header_count, sizeof(char *));
		response->header_values = calloc(response->header_count, sizeof(char *));
		NSUInteger index = 0;
		for (id key in headers) {
			NSString *keyString = [[key description] copy];
			NSString *valueString = [[headers[key] description] copy];
			response->header_keys[index] = strdup(keyString.UTF8String ?: "");
			response->header_values[index] = strdup(valueString.UTF8String ?: "");
			index++;
		}
	}
	if (data.length > 0) {
		response->body_len = data.length;
		response->body = malloc(data.length);
		memcpy(response->body, data.bytes, data.length);
	}
	return response;
}

@interface BoxAppleHTTPSessionDelegate : NSObject <NSURLSessionTaskDelegate, NSURLSessionDataDelegate>
@property(nonatomic, assign) BOOL insecure;
@property(nonatomic, assign) BOOL anchorOnly;
@property(nonatomic, strong) NSArray *anchors;
@property(nonatomic, strong) NSData *pinnedPublicKeyHashes;
@end

@implementation BoxAppleHTTPSessionDelegate

- (void)URLSession:(NSURLSession *)session
              task:(NSURLSessionTask *)task
willPerformHTTPRedirection:(NSHTTPURLResponse *)response
        newRequest:(NSURLRequest *)request
 completionHandler:(void (^)(NSURLRequest * _Nullable))completionHandler {
	completionHandler(nil);
}

- (void)URLSession:(NSURLSession *)session
              task:(NSURLSessionTask *)task
didReceiveChallenge:(NSURLAuthenticationChallenge *)challenge
 completionHandler:(void (^)(NSURLSessionAuthChallengeDisposition disposition, NSURLCredential * _Nullable credential))completionHandler {
	if (![challenge.protectionSpace.authenticationMethod isEqualToString:NSURLAuthenticationMethodServerTrust]) {
		completionHandler(NSURLSessionAuthChallengePerformDefaultHandling, nil);
		return;
	}
	SecTrustRef trustRef = challenge.protectionSpace.serverTrust;
	if (trustRef == NULL) {
		completionHandler(NSURLSessionAuthChallengeCancelAuthenticationChallenge, nil);
		return;
	}
	NSDate *verifyDate = box_apple_http_verify_date_for_request(task.currentRequest ?: task.originalRequest);
	BOOL needsCustomHandling = self.insecure || self.anchorOnly || self.anchors.count > 0 || self.pinnedPublicKeyHashes.length > 0 || verifyDate != nil;
	if (!needsCustomHandling) {
		completionHandler(NSURLSessionAuthChallengePerformDefaultHandling, nil);
		return;
	}
	BOOL ok = YES;
	if (!self.insecure) {
		ok = box_evaluate_trust(trustRef, self.anchors, self.anchorOnly, verifyDate);
	}
	if (ok && self.pinnedPublicKeyHashes.length > 0) {
		CFArrayRef certificateChain = SecTrustCopyCertificateChain(trustRef);
		SecCertificateRef leafCertificate = NULL;
		if (certificateChain != NULL && CFArrayGetCount(certificateChain) > 0) {
			leafCertificate = (SecCertificateRef)CFArrayGetValueAtIndex(certificateChain, 0);
		}
		if (leafCertificate == NULL) {
			ok = NO;
		} else {
			NSData *leafData = CFBridgingRelease(SecCertificateCopyData(leafCertificate));
			char *pinError = box_apple_http_verify_public_key_sha256(
				(uint8_t *)self.pinnedPublicKeyHashes.bytes,
				self.pinnedPublicKeyHashes.length,
				(uint8_t *)leafData.bytes,
				leafData.length
			);
			if (pinError != NULL) {
				free(pinError);
				ok = NO;
			}
		}
		if (certificateChain != NULL) {
			CFRelease(certificateChain);
		}
	}
	if (!ok) {
		completionHandler(NSURLSessionAuthChallengeCancelAuthenticationChallenge, nil);
		return;
	}
	completionHandler(NSURLSessionAuthChallengeUseCredential, [NSURLCredential credentialForTrust:trustRef]);
}

@end

@interface BoxAppleHTTPSessionHandle : NSObject
@property(nonatomic, strong) NSURLSession *session;
@property(nonatomic, strong) BoxAppleHTTPSessionDelegate *delegate;
@end

@implementation BoxAppleHTTPSessionHandle
@end

box_apple_http_session_t *box_apple_http_session_create(
	const box_apple_http_session_config_t *config,
	char **error_out
) {
	@autoreleasepool {
		NSURLSessionConfiguration *sessionConfig = [NSURLSessionConfiguration ephemeralSessionConfiguration];
		sessionConfig.URLCache = nil;
		sessionConfig.HTTPCookieStorage = nil;
		sessionConfig.URLCredentialStorage = nil;
		sessionConfig.HTTPShouldSetCookies = NO;
		if (config != NULL && config->proxy_host != NULL && config->proxy_port > 0) {
			NSMutableDictionary *proxyDictionary = [NSMutableDictionary dictionary];
			proxyDictionary[(__bridge NSString *)kCFStreamPropertySOCKSProxyHost] = [NSString stringWithUTF8String:config->proxy_host];
			proxyDictionary[(__bridge NSString *)kCFStreamPropertySOCKSProxyPort] = @(config->proxy_port);
			proxyDictionary[(__bridge NSString *)kCFStreamPropertySOCKSVersion] = (__bridge NSString *)kCFStreamSocketSOCKSVersion5;
			if (config->proxy_username != NULL) {
				proxyDictionary[(__bridge NSString *)kCFStreamPropertySOCKSUser] = [NSString stringWithUTF8String:config->proxy_username];
			}
			if (config->proxy_password != NULL) {
				proxyDictionary[(__bridge NSString *)kCFStreamPropertySOCKSPassword] = [NSString stringWithUTF8String:config->proxy_password];
			}
			sessionConfig.connectionProxyDictionary = proxyDictionary;
		}
		if (config != NULL && config->min_tls_version != 0) {
			sessionConfig.TLSMinimumSupportedProtocolVersion = (tls_protocol_version_t)config->min_tls_version;
		}
		if (config != NULL && config->max_tls_version != 0) {
			sessionConfig.TLSMaximumSupportedProtocolVersion = (tls_protocol_version_t)config->max_tls_version;
		}
		BoxAppleHTTPSessionDelegate *delegate = [[BoxAppleHTTPSessionDelegate alloc] init];
		if (config != NULL) {
			delegate.insecure = config->insecure;
			delegate.anchorOnly = config->anchor_only;
			if (config->anchors_cf != NULL) {
				delegate.anchors = (__bridge NSArray *)config->anchors_cf;
			} else {
				delegate.anchors = @[];
			}
			if (config->pinned_public_key_sha256 != NULL && config->pinned_public_key_sha256_len > 0) {
				delegate.pinnedPublicKeyHashes = [NSData dataWithBytes:config->pinned_public_key_sha256 length:config->pinned_public_key_sha256_len];
			}
		}
		NSURLSession *session = [NSURLSession sessionWithConfiguration:sessionConfig delegate:delegate delegateQueue:nil];
		if (session == nil) {
			box_set_error_string(error_out, @"create URLSession");
			return NULL;
		}
		BoxAppleHTTPSessionHandle *handle = [[BoxAppleHTTPSessionHandle alloc] init];
		handle.session = session;
		handle.delegate = delegate;
		box_apple_http_session_t *sessionHandle = calloc(1, sizeof(box_apple_http_session_t));
		sessionHandle->handle = (__bridge_retained void *)handle;
		return sessionHandle;
	}
}

void box_apple_http_session_retire(box_apple_http_session_t *session) {
	if (session == NULL || session->handle == NULL) {
		return;
	}
	BoxAppleHTTPSessionHandle *handle = (__bridge_transfer BoxAppleHTTPSessionHandle *)session->handle;
	[handle.session finishTasksAndInvalidate];
	free(session);
}

void box_apple_http_session_close(box_apple_http_session_t *session) {
	if (session == NULL || session->handle == NULL) {
		return;
	}
	BoxAppleHTTPSessionHandle *handle = (__bridge_transfer BoxAppleHTTPSessionHandle *)session->handle;
	[handle.session invalidateAndCancel];
	free(session);
}

box_apple_http_task_t *box_apple_http_session_send_async(
	box_apple_http_session_t *session,
	const box_apple_http_request_t *request,
	char **error_out
) {
	@autoreleasepool {
		if (session == NULL || session->handle == NULL || request == NULL || request->method == NULL || request->url == NULL) {
			box_set_error_string(error_out, @"invalid apple HTTP request");
			return NULL;
		}
		BoxAppleHTTPSessionHandle *handle = (__bridge BoxAppleHTTPSessionHandle *)session->handle;
		NSURL *requestURL = [NSURL URLWithString:[NSString stringWithUTF8String:request->url]];
		if (requestURL == nil) {
			box_set_error_string(error_out, @"invalid request URL");
			return NULL;
		}
		NSMutableURLRequest *urlRequest = [NSMutableURLRequest requestWithURL:requestURL];
		urlRequest.HTTPMethod = [NSString stringWithUTF8String:request->method];
		for (size_t index = 0; index < request->header_count; index++) {
			const char *key = request->header_keys[index];
			const char *value = request->header_values[index];
			if (key == NULL || value == NULL) {
				continue;
			}
			[urlRequest addValue:[NSString stringWithUTF8String:value] forHTTPHeaderField:[NSString stringWithUTF8String:key]];
		}
		if (request->body != NULL && request->body_len > 0) {
			urlRequest.HTTPBody = [NSData dataWithBytes:request->body length:request->body_len];
		}
		if (request->has_verify_time) {
			[NSURLProtocol setProperty:@(request->verify_time_unix_millis) forKey:box_apple_http_verify_time_key inRequest:urlRequest];
		}
		box_apple_http_task_t *task = calloc(1, sizeof(box_apple_http_task_t));
		dispatch_semaphore_t doneSemaphore = dispatch_semaphore_create(0);
		task->done_semaphore = (__bridge_retained void *)doneSemaphore;
		NSURLSessionDataTask *dataTask = [handle.session dataTaskWithRequest:urlRequest completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
			if (error != nil) {
				box_set_error_from_nserror(&task->error, error);
			} else if (![response isKindOfClass:[NSHTTPURLResponse class]]) {
				box_set_error_string(&task->error, @"unexpected HTTP response type");
			} else {
				task->response = box_create_response((NSHTTPURLResponse *)response, data ?: [NSData data]);
			}
			dispatch_semaphore_signal((__bridge dispatch_semaphore_t)task->done_semaphore);
		}];
		if (dataTask == nil) {
			box_set_error_string(error_out, @"create data task");
			box_apple_http_task_close(task);
			return NULL;
		}
		task->task = (__bridge_retained void *)dataTask;
		[dataTask resume];
		return task;
	}
}

box_apple_http_response_t *box_apple_http_task_wait(
	box_apple_http_task_t *task,
	char **error_out
) {
	if (task == NULL || task->done_semaphore == NULL) {
		box_set_error_string(error_out, @"invalid apple HTTP task");
		return NULL;
	}
	dispatch_semaphore_wait((__bridge dispatch_semaphore_t)task->done_semaphore, DISPATCH_TIME_FOREVER);
	if (task->error != NULL) {
		box_set_error_string(error_out, [NSString stringWithUTF8String:task->error]);
		return NULL;
	}
	return task->response;
}

void box_apple_http_task_cancel(box_apple_http_task_t *task) {
	if (task == NULL || task->task == NULL) {
		return;
	}
	NSURLSessionTask *nsTask = (__bridge NSURLSessionTask *)task->task;
	[nsTask cancel];
}

void box_apple_http_task_close(box_apple_http_task_t *task) {
	if (task == NULL) {
		return;
	}
	if (task->task != NULL) {
		__unused NSURLSessionTask *nsTask = (__bridge_transfer NSURLSessionTask *)task->task;
		task->task = NULL;
	}
	if (task->done_semaphore != NULL) {
		__unused dispatch_semaphore_t doneSemaphore = (__bridge_transfer dispatch_semaphore_t)task->done_semaphore;
		task->done_semaphore = NULL;
	}
	free(task->error);
	free(task);
}

void box_apple_http_response_free(box_apple_http_response_t *response) {
	if (response == NULL) {
		return;
	}
	for (size_t index = 0; index < response->header_count; index++) {
		free(response->header_keys[index]);
		free(response->header_values[index]);
	}
	free(response->header_keys);
	free(response->header_values);
	free(response->body);
	free(response->error);
	free(response);
}
