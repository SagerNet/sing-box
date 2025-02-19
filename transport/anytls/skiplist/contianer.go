package skiplist

// Container is a holder object that stores a collection of other objects.
type Container interface {
	IsEmpty() bool // IsEmpty checks if the container has no elements.
	Len() int      // Len returns the number of elements in the container.
	Clear()        // Clear erases all elements from the container. After this call, Len() returns zero.
}

// Map is a associative container that contains key-value pairs with unique keys.
type Map[K any, V any] interface {
	Container
	Has(K) bool                        // Checks whether the container contains element with specific key.
	Find(K) *V                         // Finds element with specific key.
	Insert(K, V)                       // Inserts a key-value pair in to the container or replace existing value.
	Remove(K) bool                     // Remove element with specific key.
	ForEach(func(K, V))                // Iterate the container.
	ForEachIf(func(K, V) bool)         // Iterate the container, stops when the callback returns false.
	ForEachMutable(func(K, *V))        // Iterate the container, *V is mutable.
	ForEachMutableIf(func(K, *V) bool) // Iterate the container, *V is mutable, stops when the callback returns false.
}

// Set is a containers that store unique elements.
type Set[K any] interface {
	Container
	Has(K) bool             // Checks whether the container contains element with specific key.
	Insert(K)               // Inserts a key-value pair in to the container or replace existing value.
	InsertN(...K)           // Inserts multiple key-value pairs in to the container or replace existing value.
	Remove(K) bool          // Remove element with specific key.
	RemoveN(...K)           // Remove multiple elements with specific keys.
	ForEach(func(K))        // Iterate the container.
	ForEachIf(func(K) bool) // Iterate the container, stops when the callback returns false.
}

// Iterator is the interface for container's iterator.
type Iterator[T any] interface {
	IsNotEnd() bool // Whether it is point to the end of the range.
	MoveToNext()    // Let it point to the next element.
	Value() T       // Return the value of current element.
}

// MapIterator is the interface for map's iterator.
type MapIterator[K any, V any] interface {
	Iterator[V]
	Key() K // The key of the element
}
