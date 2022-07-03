package linkedhashmap

import (
	"bytes"

	"github.com/goccy/go-json"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/x/list"
)

type Map[K comparable, V any] struct {
	raw    list.List[mapEntry[K, V]]
	rawMap map[K]*list.Element[mapEntry[K, V]]
}

func (m *Map[K, V]) init() {
	if m.rawMap == nil {
		m.rawMap = make(map[K]*list.Element[mapEntry[K, V]])
	}
}

func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	buffer := new(bytes.Buffer)
	buffer.WriteString("{")
	for item := m.raw.Front(); item != nil; {
		entry := item.Value
		err := json.NewEncoder(buffer).Encode(entry.Key)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(": ")
		err = json.NewEncoder(buffer).Encode(entry.Value)
		if err != nil {
			return nil, err
		}
		item = item.Next()
		if item != nil {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (m *Map[K, V]) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	m.Clear()
	m.init()
	objectStart, err := decoder.Token()
	if err != nil {
		return err
	} else if objectStart != json.Delim('{') {
		return E.New("expected json object start, but starts with ", objectStart)
	}
	for decoder.More() {
		var entryKey K
		err = decoder.Decode(&entryKey)
		if err != nil {
			return err
		}
		var entryValue V
		err = decoder.Decode(&entryValue)
		if err != nil {
			return err
		}
		m.rawMap[entryKey] = m.raw.PushBack(mapEntry[K, V]{Key: entryKey, Value: entryValue})
	}
	objectEnd, err := decoder.Token()
	if err != nil {
		return err
	} else if objectEnd != json.Delim('}') {
		return E.New("expected json object end, but ends with ", objectEnd)
	}
	return nil
}

type mapEntry[K comparable, V any] struct {
	Key   K
	Value V
}

func (m *Map[K, V]) Size() int {
	return m.raw.Size()
}

func (m *Map[K, V]) IsEmpty() bool {
	return m.raw.IsEmpty()
}

func (m *Map[K, V]) ContainsKey(key K) bool {
	m.init()
	_, loaded := m.rawMap[key]
	return loaded
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	m.init()
	value, loaded := m.rawMap[key]
	return value.Value.Value, loaded
}

func (m *Map[K, V]) Put(key K, value V) V {
	m.init()
	entry, loaded := m.rawMap[key]
	if loaded {
		oldValue := entry.Value.Value
		entry.Value.Value = value
		return oldValue
	}
	entry = m.raw.PushBack(mapEntry[K, V]{Key: key, Value: value})
	m.rawMap[key] = entry
	return common.DefaultValue[V]()
}

func (m *Map[K, V]) PutAll(other *Map[K, V]) {
	for item := other.raw.Front(); item != nil; item = item.Next() {
		m.Put(item.Value.Key, item.Value.Value)
	}
}

func (m *Map[K, V]) Remove(key K) bool {
	m.init()
	entry, loaded := m.rawMap[key]
	if !loaded {
		return false
	}
	m.raw.Remove(entry)
	delete(m.rawMap, key)
	return true
}

func (m *Map[K, V]) RemoveAll(keys []K) {
	m.init()
	for _, key := range keys {
		entry, loaded := m.rawMap[key]
		if !loaded {
			continue
		}
		m.raw.Remove(entry)
		delete(m.rawMap, key)
	}
}

func (m *Map[K, V]) AsMap() map[K]V {
	result := make(map[K]V, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result[item.Value.Key] = item.Value.Value
	}
	return result
}

func (m *Map[K, V]) Keys() []K {
	result := make([]K, 0, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result = append(result, item.Value.Key)
	}
	return result
}

func (m *Map[K, V]) Values() []V {
	result := make([]V, 0, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result = append(result, item.Value.Value)
	}
	return result
}

func (m *Map[K, V]) Clear() {
	*m = Map[K, V]{}
}
