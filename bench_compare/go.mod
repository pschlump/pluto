module github.com/pschlump/charon/bench_compare

go 1.27.0

require (
	github.com/pschlump/charon v0.0.0-00010101000000-000000000000
	github.com/pschlump/pluto v0.0.0-00010101000000-000000000000
)

// Both libraries come from the local checkouts; nothing is fetched.
replace github.com/pschlump/charon => ../

replace github.com/pschlump/pluto => ../../pluto
