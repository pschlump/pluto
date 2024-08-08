package iface_list

// sll
// sll_ts
// dll
// dll_ts

type LinearDataType[T any] interface {
	Insert(data *T)                  // same as InsertBeforeHead
	InsertBeforeHead(data *T)        //
	Append(data *T)                  // same as InsertAfterTail
	InsertAfterTail(data *T)         //
	Push(data *T)                    // same as Insert, InsertBeforeHead
	DeleteAt(int pos) (err error)    //
	Delete(data *T) (err error)      //
	DeleteFound(data *T) (err error) //
	IsEmpty() bool                   //
	Peek() (data *T, err erorr)      //
	Pop() (data *T, err erorr)       //
	Reverse()                        //
	Length() int                     //
	Truncate()                       //

	EnQueue(data *T)               // same as InsertAfterTail, sometimes called Q.Push (PushTail)
	PushTail(data *T)              // same as Append
	PopTail() (data *T, err error) // o(n) on SLL, o(1) on DLL
	ConvertToSlice() (data []*T)   // Convert to a Slice

	// InsertBeforPos
	// InsertAfterPos

	// JSON Interface Functions
}

// Implemented by sll, sll_ts, dll, dll_ts
type StackDataType interface {
	IsEmpty() bool
	Push(data *T) // same as Insert, InsertBeforeHead
	Peek() (data *T, err erorr)
	Pop() (data *T, err erorr)
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
}

// Implemented by sll, sll_ts, dll, dll_ts
type QueueDataType interface {
	IsEmpty() bool
	EnQueue(data *T) // same as InsertAfterTail, sometimes called Q.Push
	Peek() (data *T, err erorr)
	Pop() (data *T, err erorr)
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
}

// Implemented binary_tree, binary_tree_ts, avl_tree, avl_tree_ts
type TreeDataType interface {
	Insert(data *T)                       // same as InsertBeforeHead
	Delete(data *T) (err error)           //
	HasItem(data *T) (found bool)         //
	Search(data *T) (item *T, found bool) // Item will be a different pointer from data, that has IsEqual() to data
	IsEmpty() bool
	Length() int
	Truncate()
	ConvertToSlice() (data []*T) // Convert to a Slice
	FindMin() (data *T, err error)
	FindMax() (data *T, err error)
	Depth() int                //  int to get deepest part of tree
	Pop() (data *T, err error) // FindMin -> DeleteAt(0)
}

type PriorityQueueDataType interface {
	Insert(data *T)            // same as InsertBeforeHead
	Depth() int                //  int to get deepest part of tree
	Pop() (data *T, err error) // FindMin -> DeleteAt(0)
	IsEmpty() bool
	Length() int
	Truncate()
}

/*
DLL:

*	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
*	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
*	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
*	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
*	Index - return the Nth item	in the list - in a format usable with Delete.					O(n) n/2
*	InsertBeforeHead — Inserts a new element before the current first ement of list.  			O(1)
*	IsEmpty — Returns true if the linked list is empty											O(1)
*	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
*	Peek - Look at data at head of list.														O(1)
*	Pop	- Remove and return from the head of the list.											O(1)
*	Push - Insert at the head of the list.														O(1)
*	ReverseList - Reverse all the nodes in list. 												O(n)
*	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
*	ReverseWalk - Iterate from tail to head of list. 											O(n)
*	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n) n/2
*	Truncate - Delete all the nodes in list. 													O(1)
*	Walk - Iterate from head to tail of list. 													O(n)

With the basic stack operations it also can be used as a stack:
*	Push — Inserts an element at the top														O(1)
*	Pop - will remove the top element from the stack.  An error is returned if the stack is		O(1)
		empty.
*	IsEmpty — Returns true if the stack is empty												O(1)
*	Peek — Returns the top element without removing from the stack								O(1)

With the use of Enque can be used as a Queue.  This is a synonym for AppendAtTail.				O(1)

* 	PeekTail - Peek returns the last element of the DLL (like a Queue) or an error 				O(1)
		indicating that the queue is empty.
* 	PopTail - Remvoe the element at the end of the DLL.											O(1)
*	Enque - add to the tail so that DLL can be used as a Queue.									O(1)

Additional go1.22 Functionality (replacements for Walk, ReverseWalk)
All of this code can be found at the very bottom of this file. (except the type DllDeq)
This replaces the DllIter type and Front/Done/Next/Value.

*	DllSeq					The Type for the Iterator Sequence

This version of the DLL is not suitable for concurrnet usage but ../DLLTs has mutex
locks so that it is thread safe.  It has the exact same interface.

*/
