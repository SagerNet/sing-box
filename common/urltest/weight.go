package urltest

import "sync"

// WeightStorage 线程安全的权重存储实现
type WeightStorage struct {
	weights map[string]float64
	access  sync.RWMutex
}

// NewWeightStorage 创建权重存储实例
// 输出：初始化的 WeightStorage 指针
func NewWeightStorage() *WeightStorage {
	return &WeightStorage{
		weights: make(map[string]float64),
	}
}

// LoadWeight 获取指定 tag 的 outbound 权重
// 输入：tag - outbound 的标签
// 输出：权重值，是否存在
func (s *WeightStorage) LoadWeight(tag string) (float64, bool) {
	s.access.RLock()
	defer s.access.RUnlock()
	w, ok := s.weights[tag]
	return w, ok
}

// StoreWeight 存储 outbound 权重
// 输入：tag - outbound 标签，weight - 权重值
func (s *WeightStorage) StoreWeight(tag string, weight float64) {
	s.access.Lock()
	defer s.access.Unlock()
	s.weights[tag] = weight
}
