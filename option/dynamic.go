package option

type DynamicAPIOptions struct {
	// DynamicAPI服务器监听地址
	Listen string `json:"listen"`
	// API认证密钥
	Secret string `json:"secret"`
	// 是否启用配置保存
	EnableConfigSave bool `json:"enable_config_save,omitempty"`
	// 配置文件保存路径
	ConfigSavePath string `json:"config_save_path,omitempty"`
}
