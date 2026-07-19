package locale

func init() {
	localeRegistry["ru"] = &Locale{
		Locale:                  "ru",
		DeprecatedMessage:       "Использование %s устарело в sing-box %s, и эта возможность будет удалена в sing-box %s. Ознакомьтесь с руководством по миграции.",
		DeprecatedMessageNoLink: "Использование %s устарело в sing-box %s, и эта возможность будет удалена в sing-box %s.",
		InsecureFeatureMessage:  "%s считается небезопасным в графическом клиенте sing-box для %s. Чтобы использовать эту возможность, включите `Небезопасный режим` в разделе `Настройки — Ядро — Небезопасный режим`.",
		ExternalPathFeature:     "Доступ к %s (за пределами рабочего каталога) считается небезопасным в графическом клиенте sing-box для %s. Чтобы использовать эту возможность, включите `Небезопасный режим` в разделе `Настройки — Ядро — Небезопасный режим`.",
		TailscaleInitializing:   "Инициализация",
		TailscaleInUse:          "Используется другим пользователем",
		TailscaleNeedsLogin:     "Требуется вход",
		TailscaleNeedsApproval:  "Требуется подтверждение",
		TailscaleStopped:        "Остановлено",
		TailscaleStarting:       "Запуск",
		TailscaleRunning:        "Работает",
		VPNConnecting:           "Подключение",
		VPNAuthentication:       "Требуется аутентификация",
		VPNConnected:            "Подключено",
		VPNError:                "Ошибка",
		Unknown:                 "Неизвестно",
	}
}
