package option

type DynamicAPIOptions struct {
	// DynamicAPI服务器监听地址
	Listen string `json:"listen"`
	// API认证密钥
	Secret string `json:"secret"`
}
