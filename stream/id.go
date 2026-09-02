/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// AutoSeq is the sequence-part sentinel for Add: an id with Seq == AutoSeq
// asks the stream to assign the next sequence number for that millisecond
// (last.Seq+1 when the ms part equals the last ID's ms part, 0 otherwise).
// It is the "1234-*" form of Redis XADD, and — as in Redis — it is the
// internal sentinel value, so an explicit Seq of math.MaxUint64 cannot be
// added directly.
const AutoSeq uint64 = math.MaxUint64

// Errors reported by the package.  Check them with errors.Is.
var (
	// ErrInvalidID is returned by ParseID for strings that are not a
	// "ms", "ms-seq" or "ms-*" ID.
	ErrInvalidID = errors.New("stream: invalid ID string")

	// ErrIDTooSmall is returned by Add when the resolved ID is equal to
	// or smaller than the stream's last ID — entry IDs must strictly
	// increase, so 0-0 (the start sentinel) is never a valid entry ID.
	ErrIDTooSmall = errors.New("stream: ID is equal or smaller than the last ID in the stream")

	// ErrIDExhausted is returned by Add when an auto sequence number for
	// a millisecond would overflow uint64.
	ErrIDExhausted = errors.New("stream: the sequence number for this millisecond is exhausted")

	// ErrGroupExists is returned by CreateGroup when a group with that
	// name already exists on the stream.
	ErrGroupExists = errors.New("stream: consumer group already exists")
)

// Sentinel IDs for the range and read calls (the Redis "-" and "+" bounds).
// The other Redis sentinels are mapped by the caller: "$" is s.LastID()
// (XREAD/XGROUP CREATE's last-entry form) and ">" is ID{} passed to
// ReadGroup (its never-delivered form); ParseID parses only the numeric
// forms.
var (
	// MinID is the smallest possible ID, 0-0 — the Redis "-" range bound.
	// It is the start sentinel and never a valid entry ID.
	MinID = ID{}

	// MaxID is the largest possible ID — the Redis "+" range bound.
	MaxID = ID{Ms: math.MaxUint64, Seq: math.MaxUint64}
)

// ID identifies one entry of a Stream: a millisecond time part and a
// per-millisecond sequence part, compared and printed in that order.  The
// zero value is 0-0, the "start of the stream" sentinel — entry IDs
// strictly increase, so 0-0 is never a valid entry ID.  Seq may be AutoSeq
// on input to Add and ParseID ("ms-*"), never on an ID the stream assigns.
type ID struct {
	Ms  uint64
	Seq uint64
}

// CompareID returns -1 if a sorts before b, +1 if a sorts after b and 0 if
// the two are equal: Ms first, then Seq.  It is the ordering of every ID
// collection in the package — inherent in the type, like the key bytes of
// the trie packages, which is why Stream needs no comparison constructor.
func CompareID(a, b ID) int {
	switch {
	case a.Ms < b.Ms:
		return -1
	case a.Ms > b.Ms:
		return 1
	case a.Seq < b.Seq:
		return -1
	case a.Seq > b.Seq:
		return 1
	default:
		return 0
	}
}

// String returns the canonical "ms-seq" form of the ID.  An unassigned
// AutoSeq sequence part prints as the "ms-*" request form, so parsed IDs
// round-trip.
func (id ID) String() string {
	if id.Seq == AutoSeq {
		return fmt.Sprintf("%d-*", id.Ms)
	}
	return fmt.Sprintf("%d-%d", id.Ms, id.Seq)
}

// ParseID parses the numeric ID forms: "1234-56", bare "1234" (sequence
// part 0) and "1234-*" (sequence part AutoSeq, resolved by Add).  It
// rejects empty parts, signs, non-digits, values that overflow uint64 and
// more than one dash.  The Redis "-", "+", "$", ">" and bare "*" sentinels
// are caller-level concepts — see MinID, MaxID and LastID for their
// mapping.
func ParseID(s string) (ID, error) {
	dash := strings.IndexByte(s, '-')
	if dash < 0 {
		ms, err := parseIDPart(s)
		if err != nil {
			return ID{}, invalidID(s, err)
		}
		return ID{Ms: ms}, nil
	}
	ms, err := parseIDPart(s[:dash])
	if err != nil {
		return ID{}, invalidID(s, err)
	}
	if seqPart := s[dash+1:]; seqPart != "*" {
		seq, err := parseIDPart(seqPart)
		if err != nil {
			return ID{}, invalidID(s, err)
		}
		return ID{Ms: ms, Seq: seq}, nil
	}
	return ID{Ms: ms, Seq: AutoSeq}, nil
}

// parseIDPart parses one decimal uint64 part, rejecting empty strings and
// the leading sign strconv.ParseUint would otherwise accept.
func parseIDPart(s string) (uint64, error) {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return 0, fmt.Errorf("not a decimal number")
	}
	return strconv.ParseUint(s, 10, 64)
}

// invalidID wraps a part-level parse failure in ErrInvalidID.
func invalidID(s string, err error) error {
	return fmt.Errorf("%w: %q: %s", ErrInvalidID, s, err)
}

// nextID returns the smallest ID greater than id, the exclusive lower
// bound helper for "id > floor" walks.  It wraps past MaxID to MinID, so
// callers holding a possible MaxID must check that case first.
func nextID(id ID) ID {
	if id.Seq == math.MaxUint64 {
		return ID{Ms: id.Ms + 1}
	}
	return ID{Ms: id.Ms, Seq: id.Seq + 1}
}

// prevID returns the largest ID smaller than id and true, or false when id
// is MinID (nothing sorts below it).  The inclusive upper bound helper for
// "id < min" deletions.
func prevID(id ID) (ID, bool) {
	switch {
	case id.Seq > 0:
		return ID{Ms: id.Ms, Seq: id.Seq - 1}, true
	case id.Ms > 0:
		return ID{Ms: id.Ms - 1, Seq: math.MaxUint64}, true
	default:
		return ID{}, false
	}
}
