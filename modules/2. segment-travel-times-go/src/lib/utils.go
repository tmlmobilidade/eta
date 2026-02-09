package lib

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

// PrintMap takes any value and prints it as indented JSON to stdout.
// If there is an error marshaling the value to JSON, it prints the error.
//
//	@param a any - The value to print as JSON
//	@param minify ...bool - Optional parameter to minify the output
func PrintMap(a any, minify ...bool) {
	shouldMinify := false

	if len(minify) > 0 {
		shouldMinify = minify[0]
	}

	if shouldMinify {
		b, err := json.Marshal(a)
		if err != nil {
			fmt.Println("error:", err)
		}
		fmt.Printf("%s\n", string(b))
	} else {
		b, err := json.MarshalIndent(a, "", "  ")
		if err != nil {
			fmt.Println("error:", err)
		}
		fmt.Printf("%s\n", string(b))
	}
}

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

// Removes duplicates from a slice
//
//	@param slice []T - The slice to remove duplicates from
//	@return []T - The slice with duplicates removed
func RemoveDuplicates[T any](slice []T) []T {
	seen := make(map[any]bool)
	result := make([]T, 0)

	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Returns a pointer to T value
//
//	@param t T - The value to return a pointer to
//	@return *T - The pointer to the value
func Ptr[T any](t T) *T { return &t }

// Returns a hash of the values of a map
//
//	@param m map[string]any - The map to hash
//	@return string - The hash of the values of the map
func Hash(str string) string {
	return hex.EncodeToString(sha256.New().Sum([]byte(str)))
}

// GetAllStructTagValues retrieves all values of a tag from a struct
//
//	@param obj T - The struct to retrieve the tag values from
//	@param tagKey string - The tag key to retrieve the values from
//	@return []string - The values of the tag
func GetAllStructTagValues[T any](obj T, tagKey string) []string {
	v := reflect.ValueOf(obj)
	t := v.Type()

	values := make([]string, 0)

	for i := range v.NumField() {
		fieldType := t.Field(i)
		tag := fieldType.Tag.Get(tagKey)
		values = append(values, tag)
	}

	return values
}

// GetFieldByTag retrieves a field's value by its GTFS tag name from any struct
func GetFieldByTag[T any](obj *T, tagKey string, tagValue string) string {

	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)
		tag := fieldType.Tag.Get(tagKey)

		if tag == tagValue {
			return field.String()
		}
	}

	return ""
}


func SetToSlice[T comparable](set map[T]struct{}) []T {
	slice := make([]T, 0, len(set))
	for k := range set {
		slice = append(slice, k)
	}
	return slice
}