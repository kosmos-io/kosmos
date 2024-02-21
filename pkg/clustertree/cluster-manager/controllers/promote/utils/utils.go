package utils

import (
	"fmt"
	"reflect"
)

// ToMapSetE converts a slice or array to map[interface{}]interface{} with error
// interface{} is slice's item
func ToMapSetE(i interface{}) (interface{}, error) {
	// judge the validation of the input
	if i == nil {
		return nil, fmt.Errorf("unable to convert %#v of type %T to map[interface{}]interface{}", i, i)
	}
	kind := reflect.TypeOf(i).Kind()
	if kind != reflect.Slice && kind != reflect.Array {
		return nil, fmt.Errorf("the input %#v of type %T isn't a slice or array", i, i)
	}

	// execute the convert
	v := reflect.ValueOf(i)
	m := make(map[interface{}]interface{})
	for j := 0; j < v.Len(); j++ {
		value := v.Index(j).Interface()
		key := fmt.Sprintf("%v", v.Index(j).Interface())
		m[key] = value
	}
	return m, nil
}
