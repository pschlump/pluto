
all:

test:
	( cd binary_tree ; make test )
	( cd dll ; make test )
	( cd dllts ; make test )
	( cd g_lib ; make test )
	( cd queue ; make test )
	( cd sll ; make test )
	( cd sllts ; make test )
	( cd stack ; make test )