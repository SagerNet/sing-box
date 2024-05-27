// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// kanged from https://github.com/golang/mobile/blob/c713f31d574bb632a93f169b2cc99c9e753fef0e/app/android.go#L89

package include

// #include <time.h>
import "C"
import "time"

func init() {
	var currentT C.time_t
	var currentTM C.struct_tm
	C.time(&currentT)
	C.localtime_r(&currentT, &currentTM)
	tzOffset := int(currentTM.tm_gmtoff)
	tz := C.GoString(currentTM.tm_zone)
	time.Local = time.FixedZone(tz, tzOffset)
}
