// AUTO GENERATED CODE, DON'T EDIT!!!
// EDIT skiplist_newnode_generate.sh accordingly.

package skiplist

// newSkipListNode creates a new node initialized with specified key, value and next slice.
func newSkipListNode[K any, V any](level int, key K, value V) *skipListNode[K, V] {
	// For nodes with each levels, point their next slice to the nexts array allocated together,
	// which can reduce 1 memory allocation and improve performance.
	//
	// The generics of the golang doesn't support non-type parameters like in C++,
	// so we have to generate it manually.
	switch level {
	case 1:
		n := struct {
			head  skipListNode[K, V]
			nexts [1]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 2:
		n := struct {
			head  skipListNode[K, V]
			nexts [2]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 3:
		n := struct {
			head  skipListNode[K, V]
			nexts [3]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 4:
		n := struct {
			head  skipListNode[K, V]
			nexts [4]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 5:
		n := struct {
			head  skipListNode[K, V]
			nexts [5]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 6:
		n := struct {
			head  skipListNode[K, V]
			nexts [6]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 7:
		n := struct {
			head  skipListNode[K, V]
			nexts [7]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 8:
		n := struct {
			head  skipListNode[K, V]
			nexts [8]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 9:
		n := struct {
			head  skipListNode[K, V]
			nexts [9]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 10:
		n := struct {
			head  skipListNode[K, V]
			nexts [10]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 11:
		n := struct {
			head  skipListNode[K, V]
			nexts [11]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 12:
		n := struct {
			head  skipListNode[K, V]
			nexts [12]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 13:
		n := struct {
			head  skipListNode[K, V]
			nexts [13]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 14:
		n := struct {
			head  skipListNode[K, V]
			nexts [14]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 15:
		n := struct {
			head  skipListNode[K, V]
			nexts [15]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 16:
		n := struct {
			head  skipListNode[K, V]
			nexts [16]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 17:
		n := struct {
			head  skipListNode[K, V]
			nexts [17]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 18:
		n := struct {
			head  skipListNode[K, V]
			nexts [18]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 19:
		n := struct {
			head  skipListNode[K, V]
			nexts [19]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 20:
		n := struct {
			head  skipListNode[K, V]
			nexts [20]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 21:
		n := struct {
			head  skipListNode[K, V]
			nexts [21]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 22:
		n := struct {
			head  skipListNode[K, V]
			nexts [22]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 23:
		n := struct {
			head  skipListNode[K, V]
			nexts [23]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 24:
		n := struct {
			head  skipListNode[K, V]
			nexts [24]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 25:
		n := struct {
			head  skipListNode[K, V]
			nexts [25]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 26:
		n := struct {
			head  skipListNode[K, V]
			nexts [26]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 27:
		n := struct {
			head  skipListNode[K, V]
			nexts [27]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 28:
		n := struct {
			head  skipListNode[K, V]
			nexts [28]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 29:
		n := struct {
			head  skipListNode[K, V]
			nexts [29]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 30:
		n := struct {
			head  skipListNode[K, V]
			nexts [30]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 31:
		n := struct {
			head  skipListNode[K, V]
			nexts [31]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 32:
		n := struct {
			head  skipListNode[K, V]
			nexts [32]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 33:
		n := struct {
			head  skipListNode[K, V]
			nexts [33]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 34:
		n := struct {
			head  skipListNode[K, V]
			nexts [34]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 35:
		n := struct {
			head  skipListNode[K, V]
			nexts [35]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 36:
		n := struct {
			head  skipListNode[K, V]
			nexts [36]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 37:
		n := struct {
			head  skipListNode[K, V]
			nexts [37]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 38:
		n := struct {
			head  skipListNode[K, V]
			nexts [38]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 39:
		n := struct {
			head  skipListNode[K, V]
			nexts [39]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	case 40:
		n := struct {
			head  skipListNode[K, V]
			nexts [40]*skipListNode[K, V]
		}{head: skipListNode[K, V]{key, value, nil}}
		n.head.next = n.nexts[:]
		return &n.head
	}

	panic("should not reach here")
}
