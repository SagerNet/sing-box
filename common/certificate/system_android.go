//go:build android

package certificate

/*
#include <stdint.h>
#include <stdlib.h>
extern void *box_system_certificates_der(uintptr_t vm, int *out_length);
*/
import "C"

import (
	"crypto/x509"

	"github.com/sagernet/sing-box/common/jni"
)

func systemCertificates() []*x509.Certificate {
	vm := jni.VM()
	if vm == 0 {
		return nil
	}
	var length C.int
	pointer := C.box_system_certificates_der(C.uintptr_t(vm), &length)
	if pointer == nil {
		return nil
	}
	defer C.free(pointer)
	certificates, err := x509.ParseCertificates(C.GoBytes(pointer, length))
	if err != nil {
		return nil
	}
	return certificates
}
