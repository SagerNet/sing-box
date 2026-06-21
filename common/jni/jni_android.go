//go:build android

package jni

/*
#include <stdint.h>
extern uintptr_t box_jni_vm(void);
*/
import "C"

func VM() uintptr {
	return uintptr(C.box_jni_vm())
}
