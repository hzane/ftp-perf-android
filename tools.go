package main

import (
	"fmt"
	"strconv"
)

type document = map[string]interface{}

// I2I ...
func I2I(doc map[string]interface{}, names ...string) int64 {
	item := I2U(doc, names...)
	return i2i(item)
}

// I2F ...
func I2F(doc map[string]interface{}, names ...string) float64 {
	item := I2U(doc, names...)
	return i2f(item)
}

// I2Int ...
var (
	I2Float   = I2F
	I2Int     = I2I
	I2Obj     = I2O
	I2Str     = I2S
	I2Unknown = I2U
	_         = I2Obj
	_         = I2Str
	_         = I2Unknown
	_         = I2Float
)

// I2S ...
func I2S(doc document, names ...string) string {
	return i2s(I2U(doc, names...))
}

// I2Ss ...
func I2Ss(doc document, names ...string) (ret []string) {
	x := I2U(doc, names...)
	if x, ok := x.([]interface{}); ok {
		for _, x := range x {
			if x, ok := x.(string); ok {
				ret = append(ret, x)
			}
		}
	}
	return
}

// I2U ...
func I2U(doc document, names ...string) interface{} {
	if len(names) == 0 {
		return doc
	}
	parent := grand(doc, names[:len(names)-1]...)
	if parent != nil {
		v, _ := parent[names[len(names)-1]]
		return v
	}
	return nil
}

// I2O ...
func I2O(doc document, names ...string) map[string]interface{} {
	item := I2U(doc, names...)
	doc, _ = item.(document)
	return doc
}

// I2Os ...
func I2Os(doc document, names ...string) (v []map[string]interface{}) {
	item := I2U(doc, names...)
	items, _ := item.([]interface{})
	for _, item := range items {
		if s, ok := item.(document); ok {
			v = append(v, s)
		}
	}
	return v
}

func child(obj document, name string) map[string]interface{} {
	if obj != nil {
		c, _ := obj[name]
		co, _ := c.(map[string]interface{})
		return co
	}
	return nil
}

func grand(obj document, names ...string) map[string]interface{} {
	for _, name := range names {
		obj = child(obj, name)
	}
	return obj
}

func i2s(obj interface{}) string {
	switch v := obj.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprint(v)
	}
}

func i2f(obj interface{}) float64 {
	switch v := obj.(type) {
	case nil:
		return 0
	case string:
		x, _ := strconv.ParseFloat(v, 64)
		return x
	case int64:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

func i2i(obj interface{}) int64 {
	v := i2f(obj)
	return int64(v)
}
