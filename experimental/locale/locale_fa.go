package locale

func init() {
	localeRegistry["fa"] = &Locale{
		Locale:                  "fa",
		DeprecatedMessage:       "%s از sing-box %s منسوخ شده است و در sing-box %s حذف خواهد شد؛ لطفاً راهنمای مهاجرت را ببینید.",
		DeprecatedMessageNoLink: "%s از sing-box %s منسوخ شده است و در sing-box %s حذف خواهد شد.",
		InsecureFeatureMessage:  "%s در کلاینت گرافیکی sing-box برای Windows ناامن تلقی می‌شود. برای استفاده، `حالت ناامن` را در `تنظیمات - هسته - حالت ناامن` فعال کنید.",
		ExternalPathFeature:     "دسترسی به %s (خارج از پوشهٔ کاری) در کلاینت گرافیکی sing-box برای Windows ناامن تلقی می‌شود. برای استفاده، `حالت ناامن` را در `تنظیمات - هسته - حالت ناامن` فعال کنید.",
	}
}
