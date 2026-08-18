package ex1

import (
	"slices"
	"testing"
)

func TestIsEmpty(t *testing.T) {
	var tree BinaryTree[int]
	if !tree.IsEmpty() {
		t.Errorf("zero value tree should be empty")
	}
	tree.Insert(1)
	if tree.IsEmpty() {
		t.Errorf("tree with one item should not be empty")
	}
}

func TestInsert(t *testing.T) {
	var tree BinaryTree[int]
	for _, v := range []int{5, 3, 8, 1, 4, 7, 9} {
		tree.Insert(v)
	}
	var got []int
	for v := range tree.All() {
		got = append(got, v)
	}
	expected := []int{1, 3, 4, 5, 7, 8, 9}
	if !slices.Equal(got, expected) {
		t.Errorf("in-order walk: expected %v, got %v", expected, got)
	}
}

func TestInsertDuplicate(t *testing.T) {
	var tree BinaryTree[int]
	tree.Insert(5)
	tree.Insert(5)
	n := 0
	for range tree.All() {
		n++
	}
	if n != 1 {
		t.Errorf("duplicate insert should replace, walked %d items", n)
	}
}

func TestAllEmpty(t *testing.T) {
	var tree BinaryTree[int]
	for range tree.All() {
		t.Errorf("empty tree should yield no items")
	}
}

func TestAllEarlyExit(t *testing.T) {
	var tree BinaryTree[int]
	for _, v := range []int{5, 3, 8, 1, 4, 7, 9} {
		tree.Insert(v)
	}
	n := 0
	for range tree.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("early exit should stop after 1 item, walked %d", n)
	}
}

func BenchmarkInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var tree BinaryTree[int]
		for j := 0; j < 100; j++ {
			tree.Insert(j)
		}
	}
}

func BenchmarkAll(b *testing.B) {
	var tree BinaryTree[int]
	for j := 0; j < 100; j++ {
		tree.Insert(j)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range tree.All() {
		}
	}
}

/* vim: set noai ts=4 sw=4: */
