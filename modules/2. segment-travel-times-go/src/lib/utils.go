package lib

// Converts a set to a slice
//
//	@param set map[T]struct{} - The set to convert to a slice
//	@return []T - The slice
func SetToSlice[T comparable](set map[T]struct{}) []T {
	slice := make([]T, 0, len(set))
	for k := range set {
		slice = append(slice, k)
	}
	return slice
}

// Returns a pointer to T value
//
//	@param t T - The value to return a pointer to
//	@return *T - The pointer to the value
func Ptr[T any](t T) *T { return &t }

// Returns a if condition is true, otherwise returns b
// Substitute for the ternary operator
//
//	@param condition bool - The condition to check
//	@param a T - The value to return if the condition is true
//	@param b T - The value to return if the condition is false
//	@return T - The value to return
func IfThenElse[T any](condition bool, a, b T) T {
	if condition {
		return a
	}
	return b
}