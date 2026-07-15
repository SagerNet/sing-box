package locale

var warningMessageForEndUsers = "\n\n如果您不明白此消息意味着什么：您的配置文件已过时，且将很快不可用。请联系您的配置提供者以更新配置。"

func init() {
	localeRegistry["zh-Hans"] = &Locale{
		Locale:                  "zh-Hans",
		DeprecatedMessage:       "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除，请参阅迁移指南。" + warningMessageForEndUsers,
		DeprecatedMessageNoLink: "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除。" + warningMessageForEndUsers,
		InsecureFeatureMessage:  "%s 在 sing-box 的 Windows 图形客户端中被视为不安全。请在 `设置 - 核心 - 不安全模式` 中启用不安全模式后使用。",
		ExternalPathFeature:     "访问 %s（位于工作目录之外）在 sing-box 的 Windows 图形客户端中是不安全的。请在 `设置 - 核心 - 不安全模式` 中启用不安全模式后使用。",
	}
}
