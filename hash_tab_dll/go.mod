module github.com/pschlump/pluto/hash_tab

go 1.18

require (
	github.com/pschlump/MiscLib v0.0.0-20171012162159-e4e6a3a34d5f
	github.com/pschlump/godebug v1.0.1
	github.com/pschlump/pluto/comparable v0.0.2
	github.com/pschlump/pluto/dll v0.0.2
	github.com/pschlump/pluto/g_lib v0.0.2
	github.com/pschlump/pluto/sll v0.0.2
)

require (
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mattn/go-isatty v0.0.5 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/pschlump/HashStr v1.0.0 // indirect
	github.com/pschlump/json v0.0.0-20180316172947-0d2e6a308e08 // indirect
	golang.org/x/sys v0.0.0-20190222072716-a9d3bda3a223 // indirect
)

replace github.com/pschlump/pluto/comparable => ../comparable

replace github.com/pschlump/pluto/g_lib => ../g_lib

replace github.com/pschlump/pluto/sll => ../sll

replace github.com/pschlump/pluto/dll => ../dll
