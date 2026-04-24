#ifndef BOX_CERTIFICATE_ANCHORS_DARWIN_H
#define BOX_CERTIFICATE_ANCHORS_DARWIN_H

#include <stddef.h>
#include <stdint.h>

// box_certificate_anchors_from_der wraps an array of DER-encoded certificate
// blobs into a retained CFArrayRef of SecCertificateRef, returned as an opaque
// pointer. The caller owns the returned reference and must call
// box_certificate_release_anchors. Returns NULL when no blobs were accepted.
void *box_certificate_anchors_from_der(const uint8_t *const *ders, const size_t *lens, size_t count);

// box_certificate_release_anchors drops one reference from a CFArray handle
// previously returned by box_certificate_anchors_from_der. No-op on NULL.
void box_certificate_release_anchors(void *anchors);

#endif
