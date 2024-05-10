package main

import (
	"fmt"
	"sync"
)

type syncMap[K comparable, V any] struct {
	m sync.Map
}

func (m *syncMap[K, V]) String() string {
	res := ""
	m.m.Range(func(key, value any) bool {
		res += "\t" + fmt.Sprint(key.(K)) + ":" + fmt.Sprint(value.(V)) + ",\n"
		return true
	})
	return "{\n" + res + "}"
}

func (m *syncMap[K, V]) MustGet(key K) V {
	v, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero
	}
	return v.(V)
}

func (m *syncMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (m *syncMap[K, V]) Set(key K, value V) {
	m.m.Store(key, value)
}
