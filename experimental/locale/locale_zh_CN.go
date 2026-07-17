package locale

var warningMessageForEndUsers = "\n\n如果您不明白此消息意味着什么：您的配置文件已过时，且将很快不可用。请联系您的配置提供者以更新配置。"

func init() {
	localeRegistry["zh-Hans"] = &Locale{
		Locale:                  "zh-Hans",
		DeprecatedMessage:       "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除，请参阅迁移指南。" + warningMessageForEndUsers,
		DeprecatedMessageNoLink: "%s 已在 sing-box %s 中被弃用，且将在 sing-box %s 中被移除。" + warningMessageForEndUsers,
		InsecureFeatureMessage:  "%s 在 sing-box 的 Windows 图形客户端中被视为不安全。请在 `设置 - 核心 - 不安全模式` 中启用不安全模式后使用。",
		ExternalPathFeature:     "访问 %s（位于工作目录之外）在 sing-box 的 Windows 图形客户端中是不安全的。请在 `设置 - 核心 - 不安全模式` 中启用不安全模式后使用。",
		TailscaleInitializing:   "正在初始化",
		TailscaleInUse:          "正由其他用户使用",
		TailscaleNeedsLogin:     "需要登录",
		TailscaleNeedsApproval:  "需要批准",
		TailscaleStopped:        "已停止",
		TailscaleStarting:       "启动中",
		TailscaleRunning:        "运行中",
		VPNConnecting:           "正在连接",
		VPNAuthentication:       "需要认证",
		VPNConnected:            "已连接",
		VPNError:                "错误",
		Unknown:                 "未知",
	}
}
