package skiplist

// Signed is a constraint that permits any signed integer type.
// If future releases of Go add new predeclared signed integer types,
// this constraint will be modified to include them.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned is a constraint that permits any unsigned integer type.
// If future releases of Go add new predeclared unsigned integer types,
// this constraint will be modified to include them.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Integer is a constraint that permits any integer type.
// If future releases of Go add new predeclared integer types,
// this constraint will be modified to include them.
type Integer interface {
	Signed | Unsigned
}

// Float is a constraint that permits any floating-point type.
// If future releases of Go add new predeclared floating-point types,
// this constraint will be modified to include them.
type Float interface {
	~float32 | ~float64
}

// Ordered is a constraint that permits any ordered type: any type
// that supports the operators < <= >= >.
// If future releases of Go add new ordered types,
// this constraint will be modified to include them.
type Ordered interface {
	Integer | Float | ~string
}

// Numeric is a constraint that permits any numeric type.
type Numeric interface {
	Integer | Float
}

// LessFn is a function that returns whether 'a' is less than 'b'.
type LessFn[T any] func(a, b T) bool

// CompareFn is a 3 way compare function that
// returns 1  if a >  b,
// returns 0  if a == b,
// returns -1 if a < b.
type CompareFn[T any] func(a, b T) int

// HashFn is a function that returns the hash of 't'.
type HashFn[T any] func(t T) uint64

// Equals wraps the '==' operator for comparable types.
func Equals[T comparable](a, b T) bool {
	return a == b
}

// Less wraps the '<' operator for ordered types.
func Less[T Ordered](a, b T) bool {
	return a < b
}

// OrderedCompare provide default CompareFn for ordered types.
func OrderedCompare[T Ordered](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
