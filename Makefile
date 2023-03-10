
all:

test:
	( cd binary_tree ; go vet ; make test )
	( cd binary_tree_ts ; go vet ; make test )
	( cd dll ; go vet ; make test )
	( cd dllts ; go vet ; make test )
	( cd g_lib ; go vet ; make test )
	( cd queue ; go vet ; make test )
	( cd sll ; go vet ; make test )
	( cd sllts ; go vet ; make test )
	( cd stack ; go vet ; make test )
	( cd heap ; go vet ; make test )
	( cd heap_sort ; go vet ; make test )
	( cd priority_queue ; go vet ; make test )
	( cd hash_tab_bt ; go vet ; make test )
	( cd hash_tab_bt_ts ; go vet ; make test )
	( cd hash_tab_dll ; go vet ; make test )
	( cd avl_tree ; go vet ; make test )
	( cd avl_tree_ts ; go vet ; make test )

