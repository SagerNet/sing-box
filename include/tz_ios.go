package include

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
const char* getSystemTimeZone() {
    NSTimeZone *timeZone = [NSTimeZone systemTimeZone];
    NSString *timeZoneName = [timeZone description];
    return [timeZoneName UTF8String];
}
*/
import "C"

import (
	"strings"
	"time"
)

func init() {
	tzDescription := C.GoString(C.getSystemTimeZone())
	if len(tzDescription) == 0 {
		return
	}
	location, err := time.LoadLocation(strings.Split(tzDescription, " ")[0])
	if err != nil {
		return
	}
	time.Local = location
}
