/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// expectPanic runs fn and requires it to panic with a message starting
// with the given prefix (the panic messages carry a parenthesized fix
// hint after the method name).
func expectPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic %q, got none", want)
			return
		}
		got, ok := r.(string)
		if !ok || !strings.HasPrefix(got, want) {
			t.Errorf("panic = %v, want prefix %q", r, want)
		}
	}()
	fn()
}

// checkInvariants verifies the structural contracts of a stream and its
// groups through the public surface plus the internal group map: entries
// strictly ascending, entry count agreement, the last ID bounding the
// entries, and every group's PEL ascending, owned by registered
// consumers, and in agreement with Pending.  Call it after structural
// changes.  Single-goroutine tests only — it reads internals unlocked.
func checkInvariants(t *testing.T, s *Stream) {
	t.Helper()
	count := 0
	started := false
	var prev, minEntry, maxEntry ID
	for e := range s.Range(MinID, MaxID, 0) {
		if started && CompareID(prev, e.ID) >= 0 {
			t.Fatalf("entries not strictly ascending: %v then %v", prev, e.ID)
		}
		if !started {
			minEntry = e.ID
		}
		prev, maxEntry = e.ID, e.ID
		started = true
		count++
	}
	if count != s.Len() {
		t.Fatalf("Range visited %d entries, Len reports %d", count, s.Len())
	}
	if count > 0 && CompareID(maxEntry, s.last) > 0 {
		t.Fatalf("max entry %v is above the last ID %v", maxEntry, s.last)
	}
	first, hasFirst := s.FirstID()
	if hasFirst != (count > 0) || (count > 0 && first != minEntry) {
		t.Fatalf("FirstID = (%v, %v) with %d entries (min %v)", first, hasFirst, count, minEntry)
	}

	for name, g := range s.groups {
		if g.consumers == nil {
			t.Fatalf("group %q has a nil consumer set", name)
		}
		var firstPel ID
		pstarted := false
		var pprev ID
		per := map[string]int{}
		total := 0
		for pe := range g.pel.All() {
			if pstarted && CompareID(pprev, pe.id) >= 0 {
				t.Fatalf("group %q PEL not ascending: %v then %v", name, pprev, pe.id)
			}
			if _, ok := g.consumers[pe.consumer]; !ok {
				t.Fatalf("group %q PEL entry %v owned by unregistered consumer %q", name, pe.id, pe.consumer)
			}
			if pe.count < 1 {
				t.Fatalf("group %q PEL entry %v has delivery count %d", name, pe.id, pe.count)
			}
			if pe.delivery.IsZero() {
				t.Fatalf("group %q PEL entry %v has a zero delivery time", name, pe.id)
			}
			if !pstarted {
				firstPel = pe.id
			}
			pprev = pe.id
			pstarted = true
			total++
			per[pe.consumer]++
		}
		if g.pel.Len() != total {
			t.Fatalf("group %q PEL Len = %d, walked %d", name, g.pel.Len(), total)
		}
		pcount, pmin, pmax, pper := s.Pending(name)
		if pcount != total {
			t.Fatalf("group %q Pending count = %d, PEL holds %d", name, pcount, total)
		}
		if total > 0 && (pmin != firstPel || pmax != pprev) {
			t.Fatalf("group %q Pending min/max = (%v, %v), walked (%v, %v)", name, pmin, pmax, firstPel, pprev)
		}
		if len(pper) != len(per) {
			t.Fatalf("group %q Pending per-consumer = %v, walked %v", name, pper, per)
		}
		for c, n := range per {
			if pper[c] != n {
				t.Fatalf("group %q Pending per-consumer[%q] = %d, walked %d", name, c, pper[c], n)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Randomized model test.  A reference "model" re-implements the stream's
// documented semantics over plain maps and sorted slices, a fixed-seed
// RNG drives both through the same operation mix, and every step's result
// and post-state are compared.  minIdle is only ever 0 (always idle
// enough) or time.Hour (never), so the idle comparisons are deterministic
// despite real delivery times.
// ---------------------------------------------------------------------------

type modelPEL struct {
	consumer string
	count    int
}

type modelGroup struct {
	lastDelivered ID
	consumers     map[string]bool
	pel           map[ID]*modelPEL
}

type modelStream struct {
	entries map[ID][][2]string
	last    ID
	groups  map[string]*modelGroup
}

func newModelStream() *modelStream {
	return &modelStream{
		entries: map[ID][][2]string{},
		groups:  map[string]*modelGroup{},
	}
}

func (m *modelStream) sortedIDs() []ID {
	ids := make([]ID, 0, len(m.entries))
	for id := range m.entries {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, CompareID)
	return ids
}

func (m *modelStream) sortedPelIDs(g *modelGroup) []ID {
	ids := make([]ID, 0, len(g.pel))
	for id := range g.pel {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, CompareID)
	return ids
}

// modelAutoClaim mirrors AutoClaim over the model (idleOK stands in for
// minIdle: true = every entry is idle enough, false = none is).
func (m *modelStream) autoClaim(name, consumer string, idleOK bool, start ID, count int) ([]ID, ID, []ID) {
	g, ok := m.groups[name]
	if !ok {
		return nil, MinID, nil
	}
	g.consumers[consumer] = true
	limit := 0
	if count > 0 {
		limit = count * autoClaimAttempts
	}
	examined, anyExamined := 0, false
	var lastExamined ID
	var claimed, deleted []ID
	for _, id := range m.sortedPelIDs(g) {
		if CompareID(id, start) < 0 {
			continue
		}
		if limit > 0 && examined >= limit {
			break
		}
		examined++
		anyExamined = true
		lastExamined = id
		if !idleOK {
			continue
		}
		if _, exists := m.entries[id]; exists {
			pe := g.pel[id]
			pe.consumer = consumer
			pe.count++
			claimed = append(claimed, id)
			if count > 0 && len(claimed) >= count {
				break
			}
		} else {
			deleted = append(deleted, id)
			delete(g.pel, id)
		}
	}
	next := MinID
	if anyExamined {
		for _, id := range m.sortedPelIDs(g) {
			if CompareID(id, lastExamined) > 0 {
				next = id
				break
			}
		}
	}
	return claimed, next, deleted
}

// modelReadGroup mirrors ReadGroup over the model.
func (m *modelStream) readGroup(name, consumer string, after ID, count int) []ID {
	g, ok := m.groups[name]
	if !ok {
		return nil
	}
	auto := after == MinID
	floor := after
	if auto {
		floor = g.lastDelivered
	}
	g.consumers[consumer] = true
	if CompareID(floor, MaxID) >= 0 {
		return nil
	}
	var delivered []ID
	for _, id := range m.sortedIDs() {
		if CompareID(id, floor) <= 0 {
			continue
		}
		if _, pending := g.pel[id]; pending {
			continue
		}
		g.pel[id] = &modelPEL{consumer: consumer, count: 1}
		delivered = append(delivered, id)
		if count > 0 && len(delivered) >= count {
			break
		}
	}
	if auto && len(delivered) > 0 {
		g.lastDelivered = delivered[len(delivered)-1]
	}
	return delivered
}

// compareStreamModel checks every observable aspect of s against m.
func compareStreamModel(t *testing.T, s *Stream, m *modelStream) {
	t.Helper()
	checkInvariants(t, s)

	if s.Len() != len(m.entries) {
		t.Fatalf("Len = %d, model holds %d", s.Len(), len(m.entries))
	}
	if s.LastID() != m.last {
		t.Fatalf("LastID = %v, model %v", s.LastID(), m.last)
	}
	want := m.sortedIDs()
	i := 0
	for e := range s.Range(MinID, MaxID, 0) {
		if i >= len(want) || e.ID != want[i] {
			t.Fatalf("Range at %d: got %v, model %v", i, e.ID, want)
		}
		if !slices.Equal(e.Fields, m.entries[e.ID]) {
			t.Fatalf("fields of %v = %v, model %v", e.ID, e.Fields, m.entries[e.ID])
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("Range visited %d entries, model holds %d", i, len(want))
	}
	// The descending sweep agrees with the reversed ascending one.
	j := len(want) - 1
	for e := range s.RevRange(MaxID, MinID, 0) {
		if j < 0 || e.ID != want[j] {
			t.Fatalf("RevRange mismatch at %d", j)
		}
		j--
	}
	if j != -1 {
		t.Fatalf("RevRange visited %d entries, model holds %d", len(want)-1-j, len(want))
	}

	names := s.GroupNames()
	wantNames := make([]string, 0, len(m.groups))
	for name := range m.groups {
		wantNames = append(wantNames, name)
	}
	slices.Sort(wantNames)
	if !slices.Equal(names, wantNames) {
		t.Fatalf("GroupNames = %v, model %v", names, wantNames)
	}

	for name, mg := range m.groups {
		if last, ok := s.GroupLastID(name); !ok || last != mg.lastDelivered {
			t.Fatalf("group %q last-delivered = (%v, %v), model %v", name, last, ok, mg.lastDelivered)
		}
		wantCons := make([]string, 0, len(mg.consumers))
		for c := range mg.consumers {
			wantCons = append(wantCons, c)
		}
		slices.Sort(wantCons)
		if got := s.GroupConsumers(name); !slices.Equal(got, wantCons) {
			t.Fatalf("group %q consumers = %v, model %v", name, got, wantCons)
		}
		pelIDs := m.sortedPelIDs(mg)
		pes := s.PendingRange(name, "", MinID, MaxID, 0)
		if len(pes) != len(pelIDs) {
			t.Fatalf("group %q PendingRange len = %d, model %d", name, len(pes), len(pelIDs))
		}
		for k, pe := range pes {
			mpe := mg.pel[pelIDs[k]]
			if pe.ID != pelIDs[k] || pe.Consumer != mpe.consumer || pe.DeliveryCount != mpe.count {
				t.Fatalf("group %q PendingRange[%d] = %+v, model %v/%+v", name, k, pe, pelIDs[k], mpe)
			}
			if pe.DeliveryTime.IsZero() {
				t.Fatalf("group %q PEL entry %v has a zero delivery time", name, pe.ID)
			}
		}
	}
}

func TestStreamRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	s := &Stream{}
	m := newModelStream()

	groupPool := []string{"a", "b", "c"}
	consumerPool := []string{"c1", "c2", "c3"}

	randomID := func() ID {
		return ID{Ms: uint64(rng.IntN(50)), Seq: uint64(rng.IntN(8))}
	}
	pickGroup := func() string {
		if rng.IntN(10) == 0 {
			return "missing"
		}
		return groupPool[rng.IntN(len(groupPool))]
	}

	for step := 0; step < 4000; step++ {
		switch r := rng.IntN(20); {
		case r < 5: // Add — a mix of auto-seq, valid explicit and invalid IDs
			var id ID
			switch form := rng.IntN(4); form {
			case 0:
				id = ID{Ms: m.last.Ms + uint64(rng.IntN(3)), Seq: AutoSeq}
			case 1:
				id = ID{Ms: m.last.Ms + 1 + uint64(rng.IntN(3))}
			case 2:
				id = ID{Ms: m.last.Ms, Seq: m.last.Seq + 1 + uint64(rng.IntN(3))}
			default:
				id = ID{Ms: m.last.Ms, Seq: m.last.Seq} // equal: always rejected
			}
			fields := [][2]string{{"k", fmt.Sprintf("v%d", rng.IntN(5))}}
			if rng.IntN(2) == 0 {
				fields = append(fields, [2]string{"k", "dup"})
			}
			got, err := s.Add(id, fields)
			// Model the Add.
			merr := error(nil)
			if id.Seq == AutoSeq {
				if id.Ms == m.last.Ms {
					id = ID{Ms: id.Ms, Seq: m.last.Seq + 1}
				} else {
					id = ID{Ms: id.Ms}
				}
			}
			if CompareID(id, m.last) <= 0 {
				merr = ErrIDTooSmall
			} else {
				m.entries[id] = fields
				m.last = id
			}
			switch {
			case merr == nil && err != nil:
				t.Fatalf("step %d: Add(%v) unexpected error %v", step, id, err)
			case merr != nil && err == nil:
				t.Fatalf("step %d: Add(%v) should have failed", step, id)
			case err == nil && got != id:
				t.Fatalf("step %d: Add returned %v, model %v", step, got, id)
			}
		case r == 5 || r == 6: // Delete — sometimes a live entry, mostly a miss
			var id ID
			if ids := m.sortedIDs(); len(ids) > 0 && rng.IntN(3) == 0 {
				id = ids[rng.IntN(len(ids))]
			} else {
				id = randomID()
			}
			got, mgot := s.Delete(id), false
			if _, ok := m.entries[id]; ok {
				delete(m.entries, id)
				mgot = true
			}
			if got != mgot {
				t.Fatalf("step %d: Delete(%v) = %v, model %v", step, id, got, mgot)
			}
		case r == 7: // TrimMaxLen
			maxLen := rng.IntN(31)
			n := s.TrimMaxLen(maxLen)
			ids := m.sortedIDs()
			keep := len(ids) - maxLen
			if keep < 0 {
				keep = 0
			}
			for _, id := range ids[:keep] {
				delete(m.entries, id)
			}
			if n != keep {
				t.Fatalf("step %d: TrimMaxLen(%d) evicted %d, model %d", step, maxLen, n, keep)
			}
		case r == 8: // TrimMinID
			min := randomID()
			n := s.TrimMinID(min)
			mn := 0
			for _, id := range m.sortedIDs() {
				if CompareID(id, min) < 0 {
					delete(m.entries, id)
					mn++
				}
			}
			if n != mn {
				t.Fatalf("step %d: TrimMinID(%v) evicted %d, model %d", step, min, n, mn)
			}
		case r == 9: // SetLastID — strictly forward, keeping max entry <= last
			// (backwards sets are legal but unit-tested separately; here
			// they would blur the max-entry/last-ID invariant.)
			var id ID
			if rng.IntN(2) == 0 {
				id = ID{Ms: m.last.Ms + 1 + uint64(rng.IntN(4))}
			} else {
				id = ID{Ms: m.last.Ms, Seq: m.last.Seq + 1 + uint64(rng.IntN(7))}
			}
			s.SetLastID(id)
			m.last = id
		case r == 10: // CreateGroup
			name := groupPool[rng.IntN(len(groupPool))]
			start := MinID
			switch rng.IntN(4) {
			case 1:
				start = randomID()
			case 2:
				start = m.last
			case 3:
				start = MaxID
			}
			err := s.CreateGroup(name, start)
			if _, exists := m.groups[name]; !exists {
				m.groups[name] = &modelGroup{
					lastDelivered: start,
					consumers:     map[string]bool{},
					pel:           map[ID]*modelPEL{},
				}
			} else if err == nil {
				t.Fatalf("step %d: CreateGroup(%q) should have failed", step, name)
			}
		case r == 11: // DestroyGroup
			name := pickGroup()
			n := s.DestroyGroup(name)
			mn := 0
			if g, ok := m.groups[name]; ok {
				mn = len(g.pel)
				delete(m.groups, name)
			}
			if n != mn {
				t.Fatalf("step %d: DestroyGroup(%q) dropped %d, model %d", step, name, n, mn)
			}
		case r == 12 || r == 13: // ReadGroup
			name := pickGroup()
			consumer := consumerPool[rng.IntN(len(consumerPool))]
			after := MinID
			switch rng.IntN(10) {
			case 6, 7, 8:
				after = randomID()
			case 9:
				after = MaxID
			}
			count := []int{0, 1, 2, 5}[rng.IntN(4)]
			got := idsOf(s.ReadGroup(name, consumer, after, count))
			want := m.readGroup(name, consumer, after, count)
			if !slices.Equal(got, want) {
				t.Fatalf("step %d: ReadGroup(%q,%q,%v,%d) = %v, model %v", step, name, consumer, after, count, got, want)
			}
		case r == 14: // Ack — up to 3 mixed IDs
			name := pickGroup()
			var ids []ID
			if g, ok := m.groups[name]; ok && len(g.pel) > 0 {
				pel := m.sortedPelIDs(g)
				for range 1 + rng.IntN(3) {
					ids = append(ids, pel[rng.IntN(len(pel))])
				}
			}
			ids = append(ids, randomID())
			n := s.Ack(name, ids...)
			mn := 0
			if g, ok := m.groups[name]; ok {
				for _, id := range ids {
					if _, pending := g.pel[id]; pending {
						delete(g.pel, id)
						mn++
					}
				}
			}
			if n != mn {
				t.Fatalf("step %d: Ack(%q, %v) = %d, model %d", step, name, ids, n, mn)
			}
		case r == 15: // Claim
			name := pickGroup()
			consumer := consumerPool[rng.IntN(len(consumerPool))]
			idleOK := rng.IntN(2) == 0
			minIdle := time.Duration(0)
			if !idleOK {
				minIdle = time.Hour
			}
			var ids []ID
			if g, ok := m.groups[name]; ok && len(g.pel) > 0 {
				pel := m.sortedPelIDs(g)
				for range 1 + rng.IntN(3) {
					ids = append(ids, pel[rng.IntN(len(pel))])
				}
			}
			ids = append(ids, randomID())
			got := idsOf(s.Claim(name, consumer, minIdle, ids...))
			// Model the claim.
			var want []ID
			if g, ok := m.groups[name]; ok {
				g.consumers[consumer] = true
				for _, id := range ids {
					pe, pending := g.pel[id]
					if !pending || !idleOK {
						continue
					}
					if _, exists := m.entries[id]; exists {
						pe.consumer = consumer
						pe.count++
						want = append(want, id)
					} else {
						delete(g.pel, id)
					}
				}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("step %d: Claim(%q,%q,%v,%v) = %v, model %v", step, name, consumer, minIdle, ids, got, want)
			}
		case r == 16: // AutoClaim
			name := pickGroup()
			consumer := consumerPool[rng.IntN(len(consumerPool))]
			idleOK := rng.IntN(2) == 0
			minIdle := time.Duration(0)
			if !idleOK {
				minIdle = time.Hour
			}
			start := MinID
			if rng.IntN(2) == 0 {
				start = randomID()
			}
			count := []int{0, 1, 3}[rng.IntN(3)]
			gotE, gotNext, gotDel := s.AutoClaim(name, consumer, minIdle, start, count)
			wantE, wantNext, wantDel := m.autoClaim(name, consumer, idleOK, start, count)
			if !slices.Equal(idsOf(gotE), wantE) || gotNext != wantNext || !slices.Equal(gotDel, wantDel) {
				t.Fatalf("step %d: AutoClaim(%q,%q,%v,%v,%d) = (%v, %v, %v), model (%v, %v, %v)",
					step, name, consumer, minIdle, start, count,
					idsOf(gotE), gotNext, gotDel, wantE, wantNext, wantDel)
			}
		case r == 17: // GroupSetID — any direction
			name := pickGroup()
			id := randomID()
			s.GroupSetID(name, id)
			if g, ok := m.groups[name]; ok {
				g.lastDelivered = id
			}
		case r == 18: // consumer lifecycle
			name := pickGroup()
			consumer := consumerPool[rng.IntN(len(consumerPool))]
			if rng.IntN(2) == 0 {
				got := s.GroupCreateConsumer(name, consumer)
				want := false
				if g, ok := m.groups[name]; ok && !g.consumers[consumer] {
					g.consumers[consumer] = true
					want = true
				}
				if got != want {
					t.Fatalf("step %d: GroupCreateConsumer = %v, model %v", step, got, want)
				}
			} else {
				got := s.GroupDeleteConsumer(name, consumer)
				want := 0
				if g, ok := m.groups[name]; ok && g.consumers[consumer] {
					for id, pe := range g.pel {
						if pe.consumer == consumer {
							delete(g.pel, id)
							want++
						}
					}
					delete(g.consumers, consumer)
				}
				if got != want {
					t.Fatalf("step %d: GroupDeleteConsumer = %d, model %d", step, got, want)
				}
			}
		case r == 19: // trim everything occasionally, to stress regrow paths
			if rng.IntN(4) == 0 {
				n := s.TrimMaxLen(0)
				if n != len(m.entries) {
					t.Fatalf("step %d: TrimMaxLen(0) evicted %d, model %d", step, n, len(m.entries))
				}
				m.entries = map[ID][][2]string{}
			}
		}
		compareStreamModel(t, s, m)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// TestIDJSON verifies the canonical "ms-seq" string encoding of IDs and
// its round trip through ParseID.
func TestIDJSON(t *testing.T) {
	for _, id := range []ID{{Ms: 100}, {Ms: 1234, Seq: 56}, MaxID} {
		b, err := json.Marshal(id)
		if err != nil {
			t.Fatalf("json.Marshal(%v): %v", id, err)
		}
		if string(b) != `"`+id.String()+`"` {
			t.Errorf("json.Marshal(%v) = %s", id, b)
		}
		var back ID
		if err := json.Unmarshal(b, &back); err != nil || back != id {
			t.Errorf("round trip of %s got (%v, %v)", b, back, err)
		}
	}

	// The bare "ms" and "ms-*" request forms parse too.
	var id ID
	if err := json.Unmarshal([]byte(`"1234"`), &id); err != nil || id != (ID{Ms: 1234}) {
		t.Errorf(`Unmarshal("1234") = (%v, %v)`, id, err)
	}
	if err := json.Unmarshal([]byte(`"5-*"`), &id); err != nil || id != (ID{Ms: 5, Seq: AutoSeq}) {
		t.Errorf(`Unmarshal("5-*") = (%v, %v)`, id, err)
	}

	// Decode errors leave the ID untouched.
	id = ID{Ms: 7, Seq: 3}
	for _, bad := range []string{`"nope"`, `"1-2-3"`, `7`, `["1-2"]`, `{"ms":1}`} {
		if err := json.Unmarshal([]byte(bad), &id); err == nil {
			t.Errorf("Expected an error unmarshaling %s into an ID.", bad)
		}
		if id != (ID{Ms: 7, Seq: 3}) {
			t.Errorf("ID changed after the error on %s: %v", bad, id)
		}
	}
}

func TestStreamMarshalJSON(t *testing.T) {
	// Exact array output, ascending ID order.
	var s Stream
	for i := range 3 {
		if _, err := s.Add(ID{Ms: 100, Seq: AutoSeq}, [][2]string{{"sensor", fmt.Sprintf("%d", i*7)}}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"id":"100-0","fields":[["sensor","0"]]},` +
		`{"id":"100-1","fields":[["sensor","7"]]},` +
		`{"id":"100-2","fields":[["sensor","14"]]}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An empty stream encodes as [].
	if b, err := json.Marshal(&Stream{}); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty stream, got (%s, %v)", b, err)
	}

	// A direct call on a nil stream encodes as []; json.Marshal on a nil
	// *Stream never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilStream *Stream
	if b, err := nilStream.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-stream call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilStream); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil stream, got (%s, %v)", b, err)
	}

	// Only the entry log is encoded: the last assigned ID and the
	// consumer groups do not appear.
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	s.SetLastID(ID{Ms: 999})
	if b, err := json.Marshal(&s); err != nil || string(b) != want {
		t.Errorf("Groups and the last ID leaked into the encoding: (%s, %v)", b, err)
	}
}

func TestStreamUnmarshalJSON(t *testing.T) {
	// Decoded order is the ID order; the zero value is usable.
	var s Stream
	data := `[{"id":"1-0","fields":[["job","a"]]},{"id":"1-1","fields":[["job","b"]]}]`
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	got := collect(s.Range(MinID, MaxID, 0))
	if len(got) != 2 || got[0].ID != (ID{Ms: 1}) || got[1].ID != (ID{Ms: 1, Seq: 1}) ||
		got[0].Fields[0] != [2]string{"job", "a"} {
		t.Errorf("Unexpected entries after unmarshal: %v", got)
	}
	checkInvariants(t, &s)

	// The last assigned ID advances to the restored tail, so the next
	// Add starts above it.
	if s.LastID() != (ID{Ms: 1, Seq: 1}) {
		t.Errorf("LastID after unmarshal = %v", s.LastID())
	}
	if _, err := s.Add(ID{Ms: 1, Seq: AutoSeq}, nil); err != nil {
		t.Errorf("Add after unmarshal: %v", err)
	}

	// A round trip rebuilds a structurally sound stream.
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Stream
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, &again)
	if got, want := fmt.Sprint(idsOf(collect(again.Range(MinID, MaxID, 0)))),
		fmt.Sprint(idsOf(collect(s.Range(MinID, MaxID, 0)))); got != want {
		t.Errorf("Round trip got %s, want %s", got, want)
	}

	// Unmarshaling replaces the entry log; it does not append.  A higher
	// last ID (SetLastID) is not regressed.
	s.SetLastID(ID{Ms: 50})
	if err := json.Unmarshal([]byte(`[{"id":"7-0","fields":[["k","v"]]}]`), &s); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if s.Len() != 1 {
		t.Errorf("Expected replacement, got length %d", s.Len())
	}
	if s.LastID() != (ID{Ms: 50}) {
		t.Errorf("LastID regressed to %v", s.LastID())
	}

	// Replacing the log keeps the consumer groups (the Delete contract:
	// PELs are unaffected by entry removal).
	var gs Stream
	for i := range 3 {
		if _, err := gs.Add(ID{Ms: 1, Seq: uint64(i)}, [][2]string{{"job", "x"}}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := gs.CreateGroup("workers", MinID); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if n := len(gs.ReadGroup("workers", "alice", MinID, 2)); n != 2 {
		t.Fatalf("ReadGroup delivered %d", n)
	}
	if err := json.Unmarshal([]byte(`[{"id":"9-0","fields":[["job","y"]]}]`), &gs); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if names := gs.GroupNames(); !slices.Equal(names, []string{"workers"}) {
		t.Errorf("Groups lost on unmarshal: %v", names)
	}
	if count, _, _, _ := gs.Pending("workers"); count != 2 {
		t.Errorf("PEL lost on unmarshal: pending %d", count)
	}
	checkInvariants(t, &gs)

	// An empty array and null clear the entry log.
	if err := json.Unmarshal([]byte("[]"), &s); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !s.IsEmpty() {
		t.Errorf("Expected [] to clear the stream.")
	}
	if _, err := s.Add(ID{Ms: 60}, nil); err != nil { // last ID 50 kept: 60-0 is legal
		t.Fatalf("Add after clear: %v", err)
	}
	if err := json.Unmarshal([]byte("null"), &s); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !s.IsEmpty() {
		t.Errorf("Expected null to clear the stream.")
	}
	checkInvariants(t, &s)

	// Decode errors are returned and leave the stream untouched.
	keep := &Stream{}
	if _, err := keep.Add(ID{Ms: 3}, [][2]string{{"k", "keep"}}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	for _, badData := range []string{
		"[{",                             // malformed
		`{"id":"1-0"}`,                   // not an array
		"7",                              // not an array
		`[{"id":"bogus","fields":null}]`, // invalid ID
		`[{"id":7,"fields":null}]`,       // non-string ID
		`[{"id":"1-1"},{"id":"1-1"}]`,    // duplicate IDs
		`[{"id":"2-0"},{"id":"1-0"}]`,    // not increasing
		`[{"id":"0-0","fields":null}]`,   // 0-0 is never an entry ID
	} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := fmt.Sprint(idsOf(collect(keep.Range(MinID, MaxID, 0)))); got != "[3-0]" {
			t.Errorf("Stream changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, keep)
}

// TestStreamUnmarshalJSONPanics verifies that UnmarshalJSON joins the
// insert family: storing entries into a nil stream panics with the Add
// precedent's message, while [] and null — which store nothing — are
// tolerated everywhere.  (The stream has no constructor, so a zero-value
// Stream accepts data; only the nil pointer panics.)
func TestStreamUnmarshalJSONPanics(t *testing.T) {
	var nilStream *Stream
	for _, data := range []string{"[]", "null"} {
		if err := nilStream.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil stream to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "stream: UnmarshalJSON called on a nil Stream", func() {
		_ = nilStream.UnmarshalJSON([]byte(`[{"id":"1-0","fields":null}]`))
	})
}
