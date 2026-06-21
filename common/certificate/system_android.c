#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

void *box_system_certificates_der(uintptr_t vmPtr, int *out_length) {
	*out_length = 0;

	JavaVM *vm = (JavaVM *) vmPtr;
	JNIEnv *env = NULL;
	int attached = 0;
	jint getEnvResult = (*vm)->GetEnv(vm, (void **) &env, JNI_VERSION_1_6);
	if (getEnvResult == JNI_EDETACHED) {
		if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
			return NULL;
		}
		attached = 1;
	} else if (getEnvResult != JNI_OK) {
		return NULL;
	}

	unsigned char *result = NULL;
	int resultLength = 0;

	jclass keyStoreClass = (*env)->FindClass(env, "java/security/KeyStore");
	jmethodID getInstance = (*env)->GetStaticMethodID(env, keyStoreClass, "getInstance", "(Ljava/lang/String;)Ljava/security/KeyStore;");
	jstring storeName = (*env)->NewStringUTF(env, "AndroidCAStore");
	jobject keyStore = (*env)->CallStaticObjectMethod(env, keyStoreClass, getInstance, storeName);
	if ((*env)->ExceptionCheck(env) || keyStore == NULL) {
		goto done;
	}

	jmethodID load = (*env)->GetMethodID(env, keyStoreClass, "load", "(Ljava/io/InputStream;[C)V");
	(*env)->CallVoidMethod(env, keyStore, load, NULL, NULL);
	if ((*env)->ExceptionCheck(env)) {
		goto done;
	}

	jmethodID aliasesMethod = (*env)->GetMethodID(env, keyStoreClass, "aliases", "()Ljava/util/Enumeration;");
	jmethodID getCertificate = (*env)->GetMethodID(env, keyStoreClass, "getCertificate", "(Ljava/lang/String;)Ljava/security/cert/Certificate;");
	jobject aliases = (*env)->CallObjectMethod(env, keyStore, aliasesMethod);
	if ((*env)->ExceptionCheck(env) || aliases == NULL) {
		goto done;
	}

	jclass enumerationClass = (*env)->FindClass(env, "java/util/Enumeration");
	jmethodID hasMoreElements = (*env)->GetMethodID(env, enumerationClass, "hasMoreElements", "()Z");
	jmethodID nextElement = (*env)->GetMethodID(env, enumerationClass, "nextElement", "()Ljava/lang/Object;");

	jclass certificateClass = (*env)->FindClass(env, "java/security/cert/Certificate");
	jmethodID getEncoded = (*env)->GetMethodID(env, certificateClass, "getEncoded", "()[B");

	while ((*env)->CallBooleanMethod(env, aliases, hasMoreElements)) {
		jstring alias = (jstring) (*env)->CallObjectMethod(env, aliases, nextElement);
		jobject certificate = (*env)->CallObjectMethod(env, keyStore, getCertificate, alias);
		(*env)->DeleteLocalRef(env, alias);
		if ((*env)->ExceptionCheck(env) || certificate == NULL) {
			(*env)->ExceptionClear(env);
			continue;
		}
		jbyteArray encoded = (jbyteArray) (*env)->CallObjectMethod(env, certificate, getEncoded);
		(*env)->DeleteLocalRef(env, certificate);
		if ((*env)->ExceptionCheck(env) || encoded == NULL) {
			(*env)->ExceptionClear(env);
			continue;
		}
		jsize encodedLength = (*env)->GetArrayLength(env, encoded);
		unsigned char *grown = realloc(result, resultLength + encodedLength);
		if (grown == NULL) {
			(*env)->DeleteLocalRef(env, encoded);
			free(result);
			result = NULL;
			resultLength = 0;
			goto done;
		}
		result = grown;
		(*env)->GetByteArrayRegion(env, encoded, 0, encodedLength, (jbyte *) (result + resultLength));
		resultLength += encodedLength;
		(*env)->DeleteLocalRef(env, encoded);
	}

done:
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
	}
	if (attached) {
		(*vm)->DetachCurrentThread(vm);
	}
	*out_length = resultLength;
	return result;
}
