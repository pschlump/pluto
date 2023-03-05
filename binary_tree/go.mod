module github.com/pschlump/pluto/binary_tree

go 1.18

require (
	github.com/pschlump/MiscLib v0.0.0-20171012162159-e4e6a3a34d5f
	github.com/pschlump/godebug v1.0.1
	github.com/pschlump/pluto/comparable v0.0.3
	github.com/pschlump/pluto/g_lib v0.0.3
	github.com/pschlump/pluto/stack v0.0.3
)

require (
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mattn/go-isatty v0.0.5 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/pschlump/json v0.0.0-20180316172947-0d2e6a308e08 // indirect
	golang.org/x/exp v0.0.0-20230304125523-9ff063c70017 // indirect
	golang.org/x/sys v0.1.0 // indirect
)

replace github.com/pschlump/pluto/comparable => ../comparable

replace github.com/pschlump/pluto/g_lib => ../g_lib

replace github.com/pschlump/pluto/stack => ../stack
