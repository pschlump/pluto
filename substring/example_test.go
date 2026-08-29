/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package substring_test

import (
	"fmt"

	"github.com/pschlump/pluto/substring"
)

// The algs4 §5.3 trace: each algorithm finds the leftmost occurrence
// of the pattern in the same text, so all three print the same index.
// "bcara" is algs4's not-found example — a miss reports -1 (like
// strings.Index), where the Java code returns n.
func Example() {
	text := "abacadabrabracabracadabrabrabracad"
	for _, pat := range []string{"abra", "rab", "rabrabracad", "bcara"} {
		k := substring.NewKMP(pat)
		b := substring.NewBoyerMoore(pat)
		r := substring.NewRabinKarp(pat)
		fmt.Printf("pattern %q: KMP=%d BoyerMoore=%d RabinKarp=%d\n",
			pat, k.Search(text), b.Search(text), r.Search(text))
	}
	// Output:
	// pattern "abra": KMP=6 BoyerMoore=6 RabinKarp=6
	// pattern "rab": KMP=8 BoyerMoore=8 RabinKarp=8
	// pattern "rabrabracad": KMP=23 BoyerMoore=23 RabinKarp=23
	// pattern "bcara": KMP=-1 BoyerMoore=-1 RabinKarp=-1
}

// Compile the pattern once, search many texts: the searcher is
// immutable, so it can be reused (and shared between goroutines)
// freely.
func ExampleKMP_Search() {
	k := substring.NewKMP("needle")
	fmt.Println(k.Search("a needle in a haystack"))
	fmt.Println(k.Search("nothing here"))
	fmt.Println(k.Search(""))
	// Output:
	// 2
	// -1
	// -1
}

// The empty pattern matches at index 0 of every text, just like
// strings.Index.
func ExampleBoyerMoore_Search() {
	b := substring.NewBoyerMoore("")
	fmt.Println(b.Search(""))
	fmt.Println(b.Search("abc"))
	// Output:
	// 0
	// 0
}

// Rabin-Karp verifies every hash hit byte-for-byte, so its answer is
// always exact — and it works on arbitrary binary data, not just
// UTF-8.
func ExampleRabinKarp_Search() {
	r := substring.NewRabinKarp("\x00\xff")
	fmt.Println(r.Search("ab\x00\xffcd"))
	fmt.Println(r.Pattern() == "\x00\xff")
	// Output:
	// 2
	// true
}
