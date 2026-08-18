package iface_list

import (
	"errors"
	"testing"
)

// stub is a minimal in-test implementation used to verify that the
// interfaces in this package are internally consistent and implementable.
type stub[T any] struct {
	data []*T
}

var (
	_ StackDataType[int]  = (*stub[int])(nil)
	_ QueueDataType[int]  = (*stub[int])(nil)
	_ LinearDataType[int] = (*stub[int])(nil)
)

var errEmpty = errors.New("empty")

func (s *stub[T]) Insert(data *T) { s.Push(data) }
func (s *stub[T]) InsertBeforeHead(data *T) {
	s.data = append([]*T{data}, s.data...)
}
func (s *stub[T]) Append(data *T)          { s.EnQueue(data) }
func (s *stub[T]) InsertAfterTail(data *T) { s.data = append(s.data, data) }
func (s *stub[T]) Push(data *T)            { s.InsertBeforeHead(data) }
func (s *stub[T]) DeleteAt(pos int) error {
	if pos < 0 || pos >= len(s.data) {
		return errEmpty
	}
	s.data = append(s.data[:pos], s.data[pos+1:]...)
	return nil
}
func (s *stub[T]) Delete(data *T) error      { return nil }
func (s *stub[T]) DeleteFound(data *T) error { return nil }
func (s *stub[T]) IsEmpty() bool             { return len(s.data) == 0 }
func (s *stub[T]) Peek() (*T, error) {
	if s.IsEmpty() {
		return nil, errEmpty
	}
	return s.data[0], nil
}
func (s *stub[T]) Pop() (*T, error) {
	if s.IsEmpty() {
		return nil, errEmpty
	}
	rv := s.data[0]
	s.data[0] = nil
	s.data = s.data[1:]
	return rv, nil
}
func (s *stub[T]) Reverse() {}
func (s *stub[T]) Length() int {
	return len(s.data)
}
func (s *stub[T]) Truncate()        { s.data = nil }
func (s *stub[T]) EnQueue(data *T)  { s.InsertAfterTail(data) }
func (s *stub[T]) PushTail(data *T) { s.Append(data) }
func (s *stub[T]) PopTail() (*T, error) {
	if s.IsEmpty() {
		return nil, errEmpty
	}
	n := len(s.data) - 1
	rv := s.data[n]
	s.data[n] = nil
	s.data = s.data[:n]
	return rv, nil
}
func (s *stub[T]) ConvertToSlice() []*T { return s.data }

func TestStackInterface(t *testing.T) {
	var s StackDataType[int] = &stub[int]{}
	if !s.IsEmpty() {
		t.Errorf("new stack should be empty")
	}
	v := 42
	s.Push(&v)
	if s.Length() != 1 {
		t.Errorf("expected length 1, got %d", s.Length())
	}
	got, err := s.Pop()
	if err != nil || *got != 42 {
		t.Errorf("expected 42, got %v, err %v", got, err)
	}
	if _, err := s.Pop(); err == nil {
		t.Errorf("pop on empty stack should return an error")
	}
}

func TestQueueInterface(t *testing.T) {
	var q QueueDataType[int] = &stub[int]{}
	a, b := 1, 2
	q.EnQueue(&a)
	q.EnQueue(&b)
	got, err := q.Pop()
	if err != nil || *got != 1 {
		t.Errorf("FIFO order violated, got %v, err %v", got, err)
	}
	if q.Length() != 1 {
		t.Errorf("expected length 1, got %d", q.Length())
	}
}

/* vim: set noai ts=4 sw=4: */
