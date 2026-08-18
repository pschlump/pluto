// Package iface_list defines the common interfaces implemented by the
// concrete data-structure packages in this module.  Coding against these
// interfaces allows callers to swap implementations — for example between
// sll and sll_ts, the mutex-guarded thread-safe variant — without changing
// call sites.
//
// Implementations:
//
//   - LinearDataType:  sll, sll_ts, dll, dll_ts
//   - StackDataType:   sll, sll_ts, dll, dll_ts
//   - QueueDataType:   sll, sll_ts, dll, dll_ts
//   - TreeDataType:    binary_tree, binary_tree_ts, avl_tree, avl_tree_ts
//   - PriorityQueueDataType: priority_queue
//
// Thread safety is a property of the concrete implementation: the _ts
// packages are safe for concurrent use, the others are not.
package iface_list

// LinearDataType is the common interface of the linear (list-like) data
// structures.
type LinearDataType[T any] interface {
	Insert(data *T)                  // same as InsertBeforeHead
	InsertBeforeHead(data *T)        //
	Append(data *T)                  // same as InsertAfterTail
	InsertAfterTail(data *T)         //
	Push(data *T)                    // same as Insert, InsertBeforeHead
	DeleteAt(pos int) (err error)    //
	Delete(data *T) (err error)      //
	DeleteFound(data *T) (err error) //
	IsEmpty() bool                   //
	Peek() (data *T, err error)      //
	Pop() (data *T, err error)       //
	Reverse()                        //
	Length() int                     //
	Truncate()                       //

	EnQueue(data *T)               // same as InsertAfterTail, sometimes called Q.Push (PushTail)
	PushTail(data *T)              // same as Append
	PopTail() (data *T, err error) // O(n) on SLL, O(1) on DLL
	ConvertToSlice() (data []*T)   // Convert to a Slice

	// InsertBeforPos
	// InsertAfterPos

	// JSON Interface Functions
}

// StackDataType is the interface of a LIFO stack.
type StackDataType[T any] interface {
	IsEmpty() bool
	Push(data *T) // same as Insert, InsertBeforeHead
	Peek() (data *T, err error)
	Pop() (data *T, err error)
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
}

// QueueDataType is the interface of a FIFO queue.
type QueueDataType[T any] interface {
	IsEmpty() bool
	EnQueue(data *T) // same as InsertAfterTail, sometimes called Q.Push
	Peek() (data *T, err error)
	Pop() (data *T, err error)
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
}

// TreeDataType is the common interface of the tree data structures
// (binary_tree, binary_tree_ts, avl_tree, avl_tree_ts).
type TreeDataType[T any] interface {
	Insert(data *T)                       //
	Delete(data *T) (err error)           //
	HasItem(data *T) (found bool)         //
	Search(data *T) (item *T, found bool) // item is a different pointer than data, with IsEqual() true
	IsEmpty() bool
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
	FindMin() (data *T, err error)
	FindMax() (data *T, err error)
	Depth() int                // depth of the deepest part of the tree
	Pop() (data *T, err error) // FindMin -> DeleteAt(0)
}

// PriorityQueueDataType is the interface of a priority queue.
type PriorityQueueDataType[T any] interface {
	Insert(data *T)            //
	Depth() int                // depth of the deepest part of the tree
	Pop() (data *T, err error) // FindMin -> DeleteAt(0)
	IsEmpty() bool
	Length() int
	Truncate()
}

/* vim: set noai ts=4 sw=4: */
