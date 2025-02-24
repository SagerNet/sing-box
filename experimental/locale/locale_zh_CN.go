package locale

var warningMessageForEndUsers = "\n\n如果您不明白此消息意味着什么：您的配置文件已过时，且将很快不可用。请联系您的配置提供者以更新配置。"

func init() {
	localeRegistry["zh_CN"] = &Locale{
		Locale:                  "zh_CN",
		DeprecatedMessage:       "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除，请参阅迁移指南。" + warningMessageForEndUsers,
		DeprecatedMessageNoLink: "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除。" + warningMessageForEndUsers,
	}
}
