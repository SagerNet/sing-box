#import <Foundation/Foundation.h>
#import <TargetConditionals.h>

#include "device_darwin.h"

#if TARGET_OS_IOS
#import <UIKit/UIKit.h>
#include <math.h>
#elif !TARGET_OS_IPHONE
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/ps/IOPSKeys.h>
#include <IOKit/ps/IOPowerSources.h>
#endif

int boxPowerLowPowerMode(void) {
	@autoreleasepool {
		return [[NSProcessInfo processInfo] isLowPowerModeEnabled] ? 1 : 0;
	}
}

int boxPowerThermalState(void) {
	@autoreleasepool {
		return (int) [[NSProcessInfo processInfo] thermalState];
	}
}

#if TARGET_OS_IOS

__attribute__((constructor)) static void boxPowerInitBatteryMonitoring(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			[UIDevice currentDevice].batteryMonitoringEnabled = YES;
		}
	});
}

int boxPowerSource(int *level) {
	@autoreleasepool {
		*level = 0;
		__block int source = 0;
		__block float batteryLevel = -1;
		dispatch_block_t read = ^{
			UIDevice *device = [UIDevice currentDevice];
			switch (device.batteryState) {
			case UIDeviceBatteryStateUnplugged:
				source = 1;
				break;
			case UIDeviceBatteryStateCharging:
			case UIDeviceBatteryStateFull:
				source = 2;
				break;
			default:
				break;
			}
			batteryLevel = device.batteryLevel;
		};
		if ([NSThread isMainThread]) {
			read();
		} else {
			dispatch_sync(dispatch_get_main_queue(), read);
		}
		if (batteryLevel >= 0) {
			*level = (int) lroundf(batteryLevel * 100);
		}
		return source;
	}
}

#elif TARGET_OS_IPHONE

int boxPowerSource(int *level) {
	*level = 0;
	return 0;
}

#else

int boxPowerSource(int *level) {
	*level = 0;
	CFTypeRef info = IOPSCopyPowerSourcesInfo();
	if (info == NULL) {
		return 0;
	}
	CFArrayRef sources = IOPSCopyPowerSourcesList(info);
	if (sources == NULL) {
		CFRelease(info);
		return 0;
	}
	int result = 0;
	if (CFArrayGetCount(sources) > 0) {
		CFDictionaryRef source = IOPSGetPowerSourceDescription(info, CFArrayGetValueAtIndex(sources, 0));
		if (source != NULL) {
			CFStringRef state = CFDictionaryGetValue(source, CFSTR(kIOPSPowerSourceStateKey));
			if (state != NULL) {
				if (CFStringCompare(state, CFSTR(kIOPSACPowerValue), 0) == kCFCompareEqualTo) {
					result = 2;
				} else if (CFStringCompare(state, CFSTR(kIOPSBatteryPowerValue), 0) == kCFCompareEqualTo) {
					result = 1;
				}
			}
			int current = 0;
			int maximum = 0;
			CFNumberRef currentValue = CFDictionaryGetValue(source, CFSTR(kIOPSCurrentCapacityKey));
			CFNumberRef maximumValue = CFDictionaryGetValue(source, CFSTR(kIOPSMaxCapacityKey));
			if (currentValue != NULL) {
				CFNumberGetValue(currentValue, kCFNumberIntType, &current);
			}
			if (maximumValue != NULL) {
				CFNumberGetValue(maximumValue, kCFNumberIntType, &maximum);
			}
			if (maximum > 0) {
				*level = current * 100 / maximum;
			}
		}
	}
	CFRelease(sources);
	CFRelease(info);
	return result;
}

#endif
