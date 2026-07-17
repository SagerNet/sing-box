package locale

func init() {
	localeRegistry["fa"] = &Locale{
		Locale:                  "fa",
		DeprecatedMessage:       "%s از sing-box %s منسوخ شده است و در sing-box %s حذف خواهد شد؛ لطفاً راهنمای مهاجرت را ببینید.",
		DeprecatedMessageNoLink: "%s از sing-box %s منسوخ شده است و در sing-box %s حذف خواهد شد.",
		InsecureFeatureMessage:  "%s در کلاینت گرافیکی sing-box برای Windows ناامن تلقی می\u200cشود. برای استفاده، `حالت ناامن` را در `تنظیمات - هسته - حالت ناامن` فعال کنید.",
		ExternalPathFeature:     "دسترسی به %s (خارج از پوشهٔ کاری) در کلاینت گرافیکی sing-box برای Windows ناامن تلقی می\u200cشود. برای استفاده، `حالت ناامن` را در `تنظیمات - هسته - حالت ناامن` فعال کنید.",
		TailscaleInitializing:   "در حال راه\u200cاندازی",
		TailscaleInUse:          "در حال استفاده توسط کاربر دیگری",
		TailscaleNeedsLogin:     "نیاز به ورود",
		TailscaleNeedsApproval:  "نیاز به تأیید",
		TailscaleStopped:        "متوقف\u200cشده",
		TailscaleStarting:       "در حال شروع",
		TailscaleRunning:        "در حال اجرا",
		VPNConnecting:           "در حال اتصال",
		VPNAuthentication:       "نیاز به احراز هویت",
		VPNConnected:            "متصل",
		VPNError:                "خطا",
		Unknown:                 "ناشناخته",
	}
}
