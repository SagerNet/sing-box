package compatible

import "sync"

// Map is a generics sync.Map
type Map[K comparable, V any] struct {
	m sync.Map
}

func (m *Map[K, V]) Len() int {
	var count int
	m.m.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

func (m *Map[K, V]) Load(key K) (V, bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return *new(V), false
	}

	return v.(V), ok
}

func (m *Map[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

func (m *Map[K, V]) Delete(key K) {
	m.m.Delete(key)
}

func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	v, ok := m.m.LoadOrStore(key, value)
	return v.(V), ok
}

func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	v, ok := m.m.LoadAndDelete(key)
	if !ok {
		return *new(V), false
	}

	return v.(V), ok
}

func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{m: sync.Map{}}
}
