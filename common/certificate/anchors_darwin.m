#import "anchors_darwin.h"

#import <Foundation/Foundation.h>
#import <Security/Security.h>

void *box_certificate_anchors_from_der(const uint8_t *const *ders, const size_t *lens, size_t count) {
	if (count == 0 || ders == NULL || lens == NULL) {
		return NULL;
	}
	CFMutableArrayRef certificates = CFArrayCreateMutable(NULL, (CFIndex)count, &kCFTypeArrayCallBacks);
	if (certificates == NULL) {
		return NULL;
	}
	for (size_t index = 0; index < count; index++) {
		if (ders[index] == NULL || lens[index] == 0) {
			continue;
		}
		CFDataRef data = CFDataCreate(NULL, ders[index], (CFIndex)lens[index]);
		if (data == NULL) {
			continue;
		}
		SecCertificateRef certificate = SecCertificateCreateWithData(NULL, data);
		CFRelease(data);
		if (certificate == NULL) {
			continue;
		}
		CFArrayAppendValue(certificates, certificate);
		CFRelease(certificate);
	}
	if (CFArrayGetCount(certificates) == 0) {
		CFRelease(certificates);
		return NULL;
	}
	return certificates;
}

void box_certificate_release_anchors(void *anchors) {
	if (anchors == NULL) {
		return;
	}
	CFRelease((CFTypeRef)anchors);
}
