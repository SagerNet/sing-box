package adapter

// OutboundWeightStorage 存储 outbound 权重信息
// 用于 urltest_pro 查询被引用 outbound 的权重
type OutboundWeightStorage interface {
	// LoadWeight 获取指定 tag 的 outbound 权重
	// 输入：tag - outbound 的标签
	// 输出：权重值（未配置时返回 false），是否存在
	LoadWeight(tag string) (float64, bool)

	// StoreWeight 存储 outbound 权重
	// 输入：tag - outbound 标签，weight - 权重值
	StoreWeight(tag string, weight float64)
}
