package skiplist

// This implementation is based on https://github.com/liyue201/gostl/tree/master/ds/skiplist
// (many thanks), added many optimizations, such as:
//
//  - adaptive level
//  - lesser search for prevs when key already exists.
//  - reduce memory allocations
//  - richer interface.
//
// etc.

import (
	"math/bits"
	"math/rand"
	"time"
)

const (
	skipListMaxLevel = 40
)

// SkipList is a probabilistic data structure that seem likely to supplant balanced trees as the
// implementation method of choice for many applications. Skip list algorithms have the same
// asymptotic expected time bounds as balanced trees and are simpler, faster and use less space.
//
// See https://en.wikipedia.org/wiki/Skip_list for more details.
type SkipList[K any, V any] struct {
	level int                // Current level, may increase dynamically during insertion
	len   int                // Total elements numner in the skiplist.
	head  skipListNode[K, V] // head.next[level] is the head of each level.
	// This cache is used to save the previous nodes when modifying the skip list to avoid
	// allocating memory each time it is called.
	prevsCache []*skipListNode[K, V]
	rander     *rand.Rand
	impl       skipListImpl[K, V]
}

// NewSkipList creates a new SkipList for Ordered key type.
func NewSkipList[K Ordered, V any]() *SkipList[K, V] {
	sl := skipListOrdered[K, V]{}
	sl.init()
	sl.impl = (skipListImpl[K, V])(&sl)
	return &sl.SkipList
}

// NewSkipListFromMap creates a new SkipList from a map.
func NewSkipListFromMap[K Ordered, V any](m map[K]V) *SkipList[K, V] {
	sl := NewSkipList[K, V]()
	for k, v := range m {
		sl.Insert(k, v)
	}
	return sl
}

// NewSkipListFunc creates a new SkipList with specified compare function keyCmp.
func NewSkipListFunc[K any, V any](keyCmp CompareFn[K]) *SkipList[K, V] {
	sl := skipListFunc[K, V]{}
	sl.init()
	sl.keyCmp = keyCmp
	sl.impl = skipListImpl[K, V](&sl)
	return &sl.SkipList
}

// IsEmpty implements the Container interface.
func (sl *SkipList[K, V]) IsEmpty() bool {
	return sl.len == 0
}

// Len implements the Container interface.
func (sl *SkipList[K, V]) Len() int {
	return sl.len
}

// Clear implements the Container interface.
func (sl *SkipList[K, V]) Clear() {
	for i := range sl.head.next {
		sl.head.next[i] = nil
	}
	sl.level = 1
	sl.len = 0
}

// Iterate return an iterator to the skiplist.
func (sl *SkipList[K, V]) Iterate() MapIterator[K, V] {
	return &skipListIterator[K, V]{sl.head.next[0], nil}
}

// Insert inserts a key-value pair into the skiplist.
// If the key is already in the skip list, it's value will be updated.
func (sl *SkipList[K, V]) Insert(key K, value V) {
	node, prevs := sl.impl.findInsertPoint(key)

	if node != nil {
		// Already exist, update the value
		node.value = value
		return
	}

	level := sl.randomLevel()
	node = newSkipListNode(level, key, value)

	minLevel := level
	if sl.level < level {
		minLevel = sl.level
	}
	for i := 0; i < minLevel; i++ {
		node.next[i] = prevs[i].next[i]
		prevs[i].next[i] = node
	}

	if level > sl.level {
		for i := sl.level; i < level; i++ {
			sl.head.next[i] = node
		}
		sl.level = level
	}

	sl.len++
}

// Find returns the value associated with the passed key if the key is in the skiplist, otherwise
// returns nil.
func (sl *SkipList[K, V]) Find(key K) *V {
	node := sl.impl.findNode(key)
	if node != nil {
		return &node.value
	}
	return nil
}

// Has implement the Map interface.
func (sl *SkipList[K, V]) Has(key K) bool {
	return sl.impl.findNode(key) != nil
}

// LowerBound returns an iterator to the first element in the skiplist that
// does not satisfy element < value (i.e. greater or equal to),
// or a end itetator if no such element is found.
func (sl *SkipList[K, V]) LowerBound(key K) MapIterator[K, V] {
	return &skipListIterator[K, V]{sl.impl.lowerBound(key), nil}
}

// UpperBound returns an iterator to the first element in the skiplist that
// does not satisfy value < element (i.e. strictly greater),
// or a end itetator if no such element is found.
func (sl *SkipList[K, V]) UpperBound(key K) MapIterator[K, V] {
	return &skipListIterator[K, V]{sl.impl.upperBound(key), nil}
}

// FindRange returns an iterator in range [first, last) (last is not includeed).
func (sl *SkipList[K, V]) FindRange(first, last K) MapIterator[K, V] {
	return &skipListIterator[K, V]{sl.impl.lowerBound(first), sl.impl.upperBound(last)}
}

// Remove removes the key-value pair associated with the passed key and returns true if the key is
// in the skiplist, otherwise returns false.
func (sl *SkipList[K, V]) Remove(key K) bool {
	node, prevs := sl.impl.findRemovePoint(key)
	if node == nil {
		return false
	}
	for i, v := range node.next {
		prevs[i].next[i] = v
	}
	for sl.level > 1 && sl.head.next[sl.level-1] == nil {
		sl.level--
	}
	sl.len--
	return true
}

// ForEach implements the Map interface.
func (sl *SkipList[K, V]) ForEach(op func(K, V)) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		op(e.key, e.value)
	}
}

// ForEachMutable implements the Map interface.
func (sl *SkipList[K, V]) ForEachMutable(op func(K, *V)) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		op(e.key, &e.value)
	}
}

// ForEachIf implements the Map interface.
func (sl *SkipList[K, V]) ForEachIf(op func(K, V) bool) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		if !op(e.key, e.value) {
			return
		}
	}
}

// ForEachMutableIf implements the Map interface.
func (sl *SkipList[K, V]) ForEachMutableIf(op func(K, *V) bool) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		if !op(e.key, &e.value) {
			return
		}
	}
}

/// SkipList implementation part.

type skipListNode[K any, V any] struct {
	key   K
	value V
	next  []*skipListNode[K, V]
}

//go:generate bash ./skiplist_newnode_generate.sh skipListMaxLevel skiplist_newnode.go
// func newSkipListNode[K Ordered, V any](level int, key K, value V) *skipListNode[K, V]

type skipListIterator[K any, V any] struct {
	node, end *skipListNode[K, V]
}

func (it *skipListIterator[K, V]) IsNotEnd() bool {
	return it.node != it.end
}

func (it *skipListIterator[K, V]) MoveToNext() {
	it.node = it.node.next[0]
}

func (it *skipListIterator[K, V]) Key() K {
	return it.node.key
}

func (it *skipListIterator[K, V]) Value() V {
	return it.node.value
}

// skipListImpl is an interface to provide different implementation for Ordered key or CompareFn.
//
// We can use CompareFn to cumpare Ordered keys, but a separated implementation is much faster.
// We don't make the whole skip list an interface, in order to share the type independented method.
// And because these methods are called directly without going through the interface, they are also
// much faster.
type skipListImpl[K any, V any] interface {
	findNode(key K) *skipListNode[K, V]
	lowerBound(key K) *skipListNode[K, V]
	upperBound(key K) *skipListNode[K, V]
	findInsertPoint(key K) (*skipListNode[K, V], []*skipListNode[K, V])
	findRemovePoint(key K) (*skipListNode[K, V], []*skipListNode[K, V])
}

func (sl *SkipList[K, V]) init() {
	sl.level = 1
	// #nosec G404 -- This is not a security condition
	sl.rander = rand.New(rand.NewSource(time.Now().Unix()))
	sl.prevsCache = make([]*skipListNode[K, V], skipListMaxLevel)
	sl.head.next = make([]*skipListNode[K, V], skipListMaxLevel)
}

func (sl *SkipList[K, V]) randomLevel() int {
	total := uint64(1)<<uint64(skipListMaxLevel) - 1 // 2^n-1
	k := sl.rander.Uint64() % total
	level := skipListMaxLevel - bits.Len64(k) + 1
	// Since levels are randomly generated, most should be less than log2(s.len).
	// Then make a limit according to sl.len to avoid unexpectedly large value.
	for level > 3 && 1<<(level-3) > sl.len {
		level--
	}

	return level
}

/// skipListOrdered part

// skipListOrdered is the skip list implementation for Ordered types.
type skipListOrdered[K Ordered, V any] struct {
	SkipList[K, V]
}

func (sl *skipListOrdered[K, V]) findNode(key K) *skipListNode[K, V] {
	return sl.doFindNode(key, true)
}

func (sl *skipListOrdered[K, V]) doFindNode(key K, eq bool) *skipListNode[K, V] {
	// This function execute the job of findNode if eq is true, otherwise lowBound.
	// Passing the control variable eq is ugly but it's faster than testing node
	// again outside the function in findNode.
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for cur := prev.next[i]; cur != nil; cur = cur.next[i] {
			if cur.key == key {
				return cur
			}
			if cur.key > key {
				// All other node in this level must be greater than the key,
				// search the next level.
				break
			}
			prev = cur
		}
	}
	if eq {
		return nil
	}
	return prev.next[0]
}

func (sl *skipListOrdered[K, V]) lowerBound(key K) *skipListNode[K, V] {
	return sl.doFindNode(key, false)
}

func (sl *skipListOrdered[K, V]) upperBound(key K) *skipListNode[K, V] {
	node := sl.lowerBound(key)
	if node != nil && node.key == key {
		return node.next[0]
	}
	return node
}

// findInsertPoint returns (*node, nil) to the existed node if the key exists,
// or (nil, []*node) to the previous nodes if the key doesn't exist
func (sl *skipListOrdered[K, V]) findInsertPoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for next := prev.next[i]; next != nil; next = next.next[i] {
			if next.key == key {
				// The key is already existed, prevs are useless because no new node insertion.
				// stop searching.
				return next, nil
			}
			if next.key > key {
				// All other node in this level must be greater than the key,
				// search the next level.
				break
			}
			prev = next
		}
		prevs[i] = prev
	}
	return nil, prevs
}

// findRemovePoint finds the node which match the key and it's previous nodes.
func (sl *skipListOrdered[K, V]) findRemovePoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.findPrevNodes(key)
	node := prevs[0].next[0]
	if node == nil || node.key != key {
		return nil, nil
	}
	return node, prevs
}

func (sl *skipListOrdered[K, V]) findPrevNodes(key K) []*skipListNode[K, V] {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for next := prev.next[i]; next != nil; next = next.next[i] {
			if next.key >= key {
				break
			}
			prev = next
		}
		prevs[i] = prev
	}
	return prevs
}

/// skipListFunc part

// skipListFunc is the skip list implementation which compare keys with func.
type skipListFunc[K any, V any] struct {
	SkipList[K, V]
	keyCmp CompareFn[K]
}

func (sl *skipListFunc[K, V]) findNode(key K) *skipListNode[K, V] {
	node := sl.lowerBound(key)
	if node != nil && sl.keyCmp(node.key, key) == 0 {
		return node
	}
	return nil
}

func (sl *skipListFunc[K, V]) lowerBound(key K) *skipListNode[K, V] {
	var prev = &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		cur := prev.next[i]
		for ; cur != nil; cur = cur.next[i] {
			cmpRet := sl.keyCmp(cur.key, key)
			if cmpRet == 0 {
				return cur
			}
			if cmpRet > 0 {
				break
			}
			prev = cur
		}
	}
	return prev.next[0]
}

func (sl *skipListFunc[K, V]) upperBound(key K) *skipListNode[K, V] {
	node := sl.lowerBound(key)
	if node != nil && sl.keyCmp(node.key, key) == 0 {
		return node.next[0]
	}
	return node
}

// findInsertPoint returns (*node, nil) to the existed node if the key exists,
// or (nil, []*node) to the previous nodes if the key doesn't exist
func (sl *skipListFunc[K, V]) findInsertPoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for cur := prev.next[i]; cur != nil; cur = cur.next[i] {
			r := sl.keyCmp(cur.key, key)
			if r == 0 {
				// The key is already existed, prevs are useless because no new node insertion.
				// stop searching.
				return cur, nil
			}
			if r > 0 {
				// All other node in this level must be greater than the key,
				// search the next level.
				break
			}
			prev = cur
		}
		prevs[i] = prev
	}
	return nil, prevs
}

// findRemovePoint finds the node which match the key and it's previous nodes.
func (sl *skipListFunc[K, V]) findRemovePoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.findPrevNodes(key)
	node := prevs[0].next[0]
	if node == nil || sl.keyCmp(node.key, key) != 0 {
		return nil, nil
	}
	return node, prevs
}

func (sl *skipListFunc[K, V]) findPrevNodes(key K) []*skipListNode[K, V] {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for next := prev.next[i]; next != nil; next = next.next[i] {
			if sl.keyCmp(next.key, key) >= 0 {
				break
			}
			prev = next
		}
		prevs[i] = prev
	}
	return prevs
}
