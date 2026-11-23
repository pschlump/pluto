/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import "github.com/pschlump/pluto/multimap"

func (m *ListMultimap[K, V]) GetView(key K) *multimap.ValuesView[K, V] {
	return multimap.NewValuesView[K, V](m, key)
}
func (m *ListMultimap[K, V]) AsMapView() *multimap.MapView[K, V] {
	return multimap.NewMapView[K, V](m)
}
func (m *SetMultimap[K, V]) GetView(key K) *multimap.ValuesView[K, V] {
	return multimap.NewValuesView[K, V](m, key)
}
func (m *SetMultimap[K, V]) AsMapView() *multimap.MapView[K, V] {
	return multimap.NewMapView[K, V](m)
}
func (m *SortedSetMultimap[K, V]) GetView(key K) *multimap.ValuesView[K, V] {
	return multimap.NewValuesView[K, V](m, key)
}
func (m *SortedSetMultimap[K, V]) AsMapView() *multimap.MapView[K, V] {
	return multimap.NewMapView[K, V](m)
}
func (m *LinkedListMultimap[K, V]) GetView(key K) *multimap.ValuesView[K, V] {
	return multimap.NewValuesView[K, V](&m.linkedTwin, key)
}
func (m *LinkedListMultimap[K, V]) AsMapView() *multimap.MapView[K, V] {
	return multimap.NewMapView[K, V](&m.linkedTwin)
}
func (m *LinkedHashMultimap[K, V]) GetView(key K) *multimap.ValuesView[K, V] {
	return multimap.NewValuesView[K, V](&m.linkedTwin, key)
}
func (m *LinkedHashMultimap[K, V]) AsMapView() *multimap.MapView[K, V] {
	return multimap.NewMapView[K, V](&m.linkedTwin)
}
