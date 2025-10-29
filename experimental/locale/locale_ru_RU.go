package locale

var warningMessageForEndUsers = "\n\nЕсли вы не понимаете, что означает это сообщение: ваш файл конфигурации устарел и вскоре станет недоступным. Обратитесь к поставщику конфигурации, чтобы обновить ее."

func init() {
	localeRegistry["ru_RU"] = &Locale{
		DeprecatedMessage:       "%s больше не поддерживается в sing-box %s и будет удалено в sing-box %s. Обратитесь к руководству по миграции. " + warningMessageForEndUsers,
		DeprecatedMessageNoLink: "%s устарело в sing-box %s и будет удалено в sing-box %s. " + warningMessageForEndUsers,
	}
}
