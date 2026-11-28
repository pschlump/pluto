/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

// edgeLog is Graph's insertion-ordered list of live edges, stored in fixed
// blocks so that appending never recopies the whole list (a single slice of
// 260K pairs regrows at 1.25x and allocates about five times its final size).
// Invariant: every block except the last holds exactly edgeLogBlock entries,
// so position i lives at blocks[i/edgeLogBlock][i%edgeLogBlock]. The last
// block grows by doubling up to edgeLogBlock only while it is the first
// block, so small graphs stay small.
type edgeLog struct {
	blocks [][]pairKey
	n      int
}

const edgeLogBlock = 2048

// Len returns the number of entries. Complexity is O(1).
func (l *edgeLog) Len() int { return l.n }

// push appends key. Complexity is O(1) amortized.
func (l *edgeLog) push(key pairKey) {
	last := len(l.blocks) - 1
	if last < 0 || len(l.blocks[last]) == edgeLogBlock {
		size := 16 // only the first block starts small
		if last >= 0 {
			size = edgeLogBlock
		}
		l.blocks = append(l.blocks, make([]pairKey, 0, size))
		last++
	}
	block := l.blocks[last]
	if len(block) == cap(block) {
		grown := make([]pairKey, len(block), min(edgeLogBlock, 2*cap(block)))
		copy(grown, block)
		block = grown
	}
	l.blocks[last] = append(block, key)
	l.n++
}

// all calls yield for each entry in order until it returns false.
func (l *edgeLog) all(yield func(int, pairKey) bool) {
	i := 0
	for _, block := range l.blocks {
		for _, key := range block {
			if !yield(i, key) {
				return
			}
			i++
		}
	}
}

// deleteAt removes position i, shifting later entries down by one so the
// order and the full-blocks invariant are kept. Complexity is O(n - i).
func (l *edgeLog) deleteAt(i int) {
	b, o := i/edgeLogBlock, i%edgeLogBlock
	for ; b < len(l.blocks); b, o = b+1, 0 {
		block := l.blocks[b]
		copy(block[o:], block[o+1:])
		if b+1 < len(l.blocks) {
			block[len(block)-1] = l.blocks[b+1][0]
		}
	}
	l.truncate(l.n - 1)
}

// deleteFunc removes every entry for which drop reports true, keeping order.
// Complexity is O(n).
func (l *edgeLog) deleteFunc(drop func(pairKey) bool) {
	w := 0
	for b := range l.blocks {
		for _, key := range l.blocks[b] {
			if drop(key) {
				continue
			}
			l.blocks[w/edgeLogBlock][w%edgeLogBlock] = key
			w++
		}
	}
	l.truncate(w)
}

// truncate shortens the log to n entries, releasing emptied blocks.
func (l *edgeLog) truncate(n int) {
	full := (n + edgeLogBlock - 1) / edgeLogBlock
	clear(l.blocks[full:])
	l.blocks = l.blocks[:full]
	if full > 0 {
		last := l.blocks[full-1]
		keep := n - (full-1)*edgeLogBlock
		l.blocks[full-1] = last[:keep]
	}
	l.n = n
}

// clone returns an independent copy backed by one allocation; each block is
// capped at its own end so appends to the copy never overlap.
func (l *edgeLog) clone() edgeLog {
	if l.n == 0 {
		return edgeLog{}
	}
	backing := make([]pairKey, l.n)
	result := edgeLog{blocks: make([][]pairKey, len(l.blocks)), n: l.n}
	for b, block := range l.blocks {
		n := copy(backing, block)
		result.blocks[b] = backing[:n:n]
		backing = backing[n:]
	}
	return result
}
