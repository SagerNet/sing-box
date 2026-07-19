package locale

func init() {
	localeRegistry["zh-Hant"] = &Locale{
		Locale:                  "zh-Hant",
		DeprecatedMessage:       "%s 已在 sing-box %s 中棄用，且將在 sing-box %s 中移除，請參閱遷移指南。",
		DeprecatedMessageNoLink: "%s 已在 sing-box %s 中棄用，且將在 sing-box %s 中移除。",
		InsecureFeatureMessage:  "%s 在 sing-box 的 %s 圖形用戶端中被視為不安全。請在 `設置 - 核心 - 不安全模式` 中啟用不安全模式後使用。",
		ExternalPathFeature:     "存取 %s（位於工作目錄之外）在 sing-box 的 %s 圖形用戶端中被視為不安全。請在 `設置 - 核心 - 不安全模式` 中啟用不安全模式後使用。",
		TailscaleInitializing:   "正在初始化",
		TailscaleInUse:          "正由其他使用者使用",
		TailscaleNeedsLogin:     "需要登入",
		TailscaleNeedsApproval:  "需要核准",
		TailscaleStopped:        "已停止",
		TailscaleStarting:       "啟動中",
		TailscaleRunning:        "執行中",
		VPNConnecting:           "正在連線",
		VPNAuthentication:       "需要認證",
		VPNConnected:            "已連線",
		VPNError:                "錯誤",
		Unknown:                 "未知",
	}
}
