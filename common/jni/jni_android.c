#include <jni.h>
#include <stdint.h>

static JavaVM *javaVM;

JNIEXPORT jint JNI_OnLoad(JavaVM *vm, void *reserved) {
	javaVM = vm;
	return JNI_VERSION_1_6;
}

uintptr_t box_jni_vm(void) {
	return (uintptr_t) javaVM;
}
