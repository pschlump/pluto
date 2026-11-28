/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"errors"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pschlump/pluto/bimap"
	"github.com/pschlump/pluto/multimap"
)

// weakHash forces long probe chains and repeated growth in the func-mode
// table: only eight distinct hash values exist.
func weakHash(value int) uint64 { return uint64(value&7) * 0x9e3779b97f4a7c15 }

func intEqual(a, b int) bool { return a == b }

// TestHashIndexModesAgainstModel drives both index modes through random
// getOrAdd/addUnique/lookup sequences and compares every answer with a builtin
// map model. Fixed seed.
func TestHashIndexModesAgainstModel(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for round := 0; round < 40; round++ {
		hint := random.Intn(64)
		indexes := map[string]*hashIndex[int]{
			"natural":   newNaturalIndex[int](hint),
			"func":      newHashIndexSized(intEqual, weakHash, hint),
			"funcEmpty": newHashIndex(intEqual, weakHash),
		}
		for name, index := range indexes {
			model := map[int]int{}
			span := 1 + random.Intn(300)
			for step := 0; step < 600; step++ {
				key := random.Intn(span)
				switch random.Intn(3) {
				case 0:
					position, found := index.getOrAdd(key, step)
					want, wantFound := model[key]
					if !wantFound {
						model[key], want = step, step
					}
					if found != wantFound || position != want {
						t.Fatalf("%s round %d: getOrAdd(%d) = %d,%v want %d,%v", name, round, key, position, found, want, wantFound)
					}
				case 1:
					_, present := model[key]
					if present {
						continue // addUnique may overwrite on a duplicate; constructors abort there
					}
					if !index.addUnique(key, step) {
						t.Fatalf("%s round %d: addUnique(%d) reported a duplicate", name, round, key)
					}
					model[key] = step
				default:
					position, found := index.get(key)
					want, wantFound := model[key]
					if found != wantFound || (found && position != want) {
						t.Fatalf("%s round %d: get(%d) = %d,%v want %d,%v", name, round, key, position, found, want, wantFound)
					}
				}
			}
			for key, want := range model {
				if position, found := index.get(key); !found || position != want {
					t.Fatalf("%s round %d: final get(%d) = %d,%v want %d", name, round, key, position, found, want)
				}
			}
			if _, found := index.get(span + 1); found {
				t.Fatalf("%s round %d: absent key found", name, round)
			}
		}
	}
	var nilIndex *hashIndex[int]
	if _, found := nilIndex.get(1); found {
		t.Fatal("nil index found a key")
	}
}

func TestHashIndexAddUniqueReportsDuplicate(t *testing.T) {
	for name, index := range map[string]*hashIndex[int]{
		"natural": newNaturalIndex[int](0),
		"func":    newHashIndex(intEqual, weakHash),
	} {
		if !index.addUnique(5, 0) || index.addUnique(5, 1) {
			t.Fatalf("%s: addUnique duplicate detection wrong", name)
		}
		if !index.add(6, 2) || index.add(6, 3) {
			t.Fatalf("%s: add duplicate detection wrong", name)
		}
		if position, _ := index.get(6); position != 2 {
			t.Fatalf("%s: add replaced the first position: %d", name, position)
		}
	}
}

// TestHashFamiliesNaturalFuncAndModelAgree builds every hash family from the
// same random input through the natural constructor, the Func constructor
// (weak hash), both builders, and a simple slice/map model, then compares
// order, lookups and duplicate errors. Fixed seed.
func TestHashFamiliesNaturalFuncAndModelAgree(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 60; round++ {
		size := random.Intn(200)
		span := 1 + random.Intn(400)
		values := make([]int, size)
		for index := range values {
			values[index] = random.Intn(span)
		}

		// Model: first-occurrence order, counts, grouped values.
		var distinct []int
		counts := map[int]int{}
		groups := map[int][]int{}
		for position, value := range values {
			if counts[value] == 0 {
				distinct = append(distinct, value)
			}
			counts[value]++
			groups[value] = append(groups[value], position)
		}

		setBuilder := NewSetBuilder[int]()
		setBuilder.AddAll(values)
		funcSetBuilder := NewSetBuilderFunc(intEqual, weakHash)
		funcSetBuilder.AddAll(values)
		for name, set := range map[string]*Set[int]{
			"natural": NewSet(values), "func": NewSetFunc(intEqual, weakHash, values),
			"builder": setBuilder.Build(), "funcBuilder": funcSetBuilder.Build(),
		} {
			if got := set.Slice(); !slices.Equal(got, distinct) && (len(got) != 0 || len(distinct) != 0) {
				t.Fatalf("round %d set %s order = %v want %v", round, name, got, distinct)
			}
			for probe := -1; probe <= span; probe++ {
				if set.Contains(probe) != (counts[probe] > 0) {
					t.Fatalf("round %d set %s Contains(%d) wrong", round, name, probe)
				}
			}
		}

		multisetBuilder := NewMultisetBuilder[int]()
		multisetBuilder.AddAll(values)
		for name, multiset := range map[string]*Multiset[int]{
			"natural": NewMultiset(values), "func": NewMultisetFunc(intEqual, weakHash, values),
			"builder": multisetBuilder.Build(),
		} {
			if multiset.Len() != size || multiset.DistinctLen() != len(distinct) {
				t.Fatalf("round %d multiset %s sizes %d/%d", round, name, multiset.Len(), multiset.DistinctLen())
			}
			position := 0
			for entry := range multiset.Entries() {
				if entry.Value != distinct[position] || entry.Count != counts[entry.Value] {
					t.Fatalf("round %d multiset %s entry %d = %+v", round, name, position, entry)
				}
				position++
			}
			for probe := -1; probe <= span; probe++ {
				if multiset.Count(probe) != counts[probe] {
					t.Fatalf("round %d multiset %s Count(%d) wrong", round, name, probe)
				}
			}
		}

		associations := make([]multimap.Association[int, int], size)
		for position, value := range values {
			associations[position] = multimap.Association[int, int]{Key: value, Value: position}
		}
		multimapBuilder := NewMultimapBuilder[int, int]()
		multimapBuilder.PutAll(associations)
		for name, value := range map[string]*Multimap[int, int]{
			"natural": NewMultimap(associations), "func": NewMultimapFunc(intEqual, weakHash, intEqual, associations),
			"builder": multimapBuilder.Build(),
		} {
			if value.Len() != size || value.KeyLen() != len(distinct) {
				t.Fatalf("round %d multimap %s sizes", round, name)
			}
			if got := collect(value.KeySet()); !slices.Equal(got, distinct) {
				t.Fatalf("round %d multimap %s keys = %v want %v", round, name, got, distinct)
			}
			var wantEntries []multimap.Association[int, int]
			for _, key := range distinct {
				for _, position := range groups[key] {
					wantEntries = append(wantEntries, multimap.Association[int, int]{Key: key, Value: position})
				}
			}
			if got := value.EntriesSlice(); !slices.Equal(got, wantEntries) {
				t.Fatalf("round %d multimap %s entries differ", round, name)
			}
			for probe := -1; probe <= span; probe++ {
				got, found := value.Get(probe)
				if found != (counts[probe] > 0) || !slices.Equal(got, groups[probe]) {
					t.Fatalf("round %d multimap %s Get(%d) = %v,%v want %v", round, name, probe, got, found, groups[probe])
				}
				if value.ContainsKey(probe) != found {
					t.Fatalf("round %d multimap %s ContainsKey(%d) wrong", round, name, probe)
				}
				if found && (!value.ContainsEntry(probe, groups[probe][0]) || value.ContainsEntry(probe, -1)) {
					t.Fatalf("round %d multimap %s ContainsEntry(%d) wrong", round, name, probe)
				}
			}
		}

		// Map and BiMap: duplicates must fail at the first repeated position.
		mapEntries := make([]Entry[int, int], size)
		biEntries := make([]bimap.Association[int, int], size)
		firstDuplicate := -1
		seen := map[int]bool{}
		for position, value := range values {
			mapEntries[position] = Entry[int, int]{Key: value, Value: position * 3}
			biEntries[position] = bimap.Association[int, int]{Key: value, Value: position * 3}
			if seen[value] && firstDuplicate < 0 {
				firstDuplicate = position
			}
			seen[value] = true
		}
		naturalMap, naturalErr := NewMap(mapEntries)
		funcMap, funcErr := NewMapFunc(intEqual, weakHash, mapEntries)
		naturalBi, naturalBiErr := NewBiMap(biEntries)
		funcBi, funcBiErr := NewBiMapFunc(intEqual, weakHash, intEqual, weakHash, biEntries)
		if firstDuplicate >= 0 {
			for name, err := range map[string]error{"map": naturalErr, "funcMap": funcErr, "bimap": naturalBiErr, "funcBimap": funcBiErr} {
				if !errors.Is(err, ErrDuplicateKey) || !containsPosition(err, firstDuplicate) {
					t.Fatalf("round %d %s duplicate error = %v want position %d", round, name, err, firstDuplicate)
				}
			}
			continue
		}
		for name, value := range map[string]*Map[int, int]{"natural": naturalMap, "func": funcMap} {
			if got := value.EntriesSlice(); !slices.Equal(got, mapEntries) && (len(got) != 0 || size != 0) {
				t.Fatalf("round %d map %s entries differ", round, name)
			}
			for probe := -1; probe <= span; probe++ {
				got, found := value.Get(probe)
				if found != seen[probe] || (found && got != mapEntries[slices.Index(values, probe)].Value) {
					t.Fatalf("round %d map %s Get(%d) wrong", round, name, probe)
				}
			}
		}
		for name, value := range map[string]*BiMap[int, int]{"natural": naturalBi, "func": funcBi} {
			if got := value.EntriesSlice(); !slices.Equal(got, biEntries) && (len(got) != 0 || size != 0) {
				t.Fatalf("round %d bimap %s entries differ", round, name)
			}
			inverse := value.Inverse()
			position := 0
			for entry := range inverse.Entries() {
				if entry.Key != biEntries[position].Value || entry.Value != biEntries[position].Key {
					t.Fatalf("round %d bimap %s inverse entry %d = %+v", round, name, position, entry)
				}
				position++
			}
			if position != size || !slices.Equal(collect(inverse.Keys()), collect(value.Values())) || !slices.Equal(collect(inverse.Values()), collect(value.Keys())) {
				t.Fatalf("round %d bimap %s inverse views differ", round, name)
			}
			for _, entry := range biEntries {
				if got, found := value.GetValue(entry.Key); !found || got != entry.Value {
					t.Fatalf("round %d bimap %s GetValue(%d) wrong", round, name, entry.Key)
				}
				if got, found := value.GetKey(entry.Value); !found || got != entry.Key {
					t.Fatalf("round %d bimap %s GetKey(%d) wrong", round, name, entry.Value)
				}
			}
			if _, found := value.GetValue(-1); found {
				t.Fatalf("round %d bimap %s found absent key", round, name)
			}
		}
	}
}

func containsPosition(err error, position int) bool {
	return err != nil && hasSuffixNumber(err.Error(), position)
}

func hasSuffixNumber(message string, position int) bool {
	return strings.HasSuffix(message, " at entry "+strconv.Itoa(position))
}

// TestBiMapDuplicateValueReportedAfterKeyCheck pins the per-entry check order:
// a key duplicate at an entry wins over a value duplicate at the same entry.
func TestBiMapDuplicateValueReportedAfterKeyCheck(t *testing.T) {
	entries := []bimap.Association[string, int]{{Key: "a", Value: 1}, {Key: "b", Value: 2}, {Key: "c", Value: 1}}
	if _, err := NewBiMap(entries); !errors.Is(err, ErrDuplicateValue) || !hasSuffixNumber(err.Error(), 2) {
		t.Fatalf("value duplicate error = %v", err)
	}
	entries[2] = bimap.Association[string, int]{Key: "a", Value: 1}
	if _, err := NewBiMap(entries); !errors.Is(err, ErrDuplicateKey) || !hasSuffixNumber(err.Error(), 2) {
		t.Fatalf("key duplicate error = %v", err)
	}
}

// TestMultimapSharedGroupsAreNotExposed proves the single shared backing array
// cannot be reached or mutated through Get or AsMap: writes and appends to the
// returned slices change neither this key's group nor its neighbour's.
func TestMultimapSharedGroupsAreNotExposed(t *testing.T) {
	value := NewMultimap([]multimap.Association[string, int]{
		{Key: "a", Value: 1}, {Key: "b", Value: 10}, {Key: "a", Value: 2}, {Key: "b", Value: 20},
	})
	got, _ := value.Get("a")
	got[0] = 99
	got = append(got, 100, 101) // would clobber "b" if Get returned the shared group
	_ = got
	for key, values := range value.AsMap() {
		values[0] = -1
		_ = append(values, -2)
		_ = key
	}
	if a, _ := value.Get("a"); !slices.Equal(a, []int{1, 2}) {
		t.Fatalf("group a mutated: %v", a)
	}
	if b, _ := value.Get("b"); !slices.Equal(b, []int{10, 20}) {
		t.Fatalf("group b mutated: %v", b)
	}
	if got := value.EntriesSlice(); len(got) != 4 || got[1] != (multimap.Association[string, int]{Key: "a", Value: 2}) {
		t.Fatalf("entries mutated: %v", got)
	}
}

// TestBiMapInverseSharingIsReadOnly checks that the inverse, which shares the
// key and value slices, hands out independent EntriesSlice copies.
func TestBiMapInverseSharingIsReadOnly(t *testing.T) {
	value, err := NewBiMap([]bimap.Association[string, int]{{Key: "a", Value: 1}, {Key: "b", Value: 2}})
	if err != nil {
		t.Fatal(err)
	}
	entries := value.Inverse().EntriesSlice()
	entries[0].Key, entries[0].Value = 9, "z"
	forward := value.EntriesSlice()
	forward[1].Key = "y"
	if got, _ := value.GetValue("a"); got != 1 {
		t.Fatalf("forward mutated: %d", got)
	}
	if got := value.Inverse().EntriesSlice(); got[0] != (bimap.Association[int, string]{Key: 1, Value: "a"}) {
		t.Fatalf("inverse mutated: %v", got)
	}
	if got := collect(value.Keys()); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("keys mutated: %v", got)
	}
	empty, _ := NewBiMap[string, int](nil)
	if empty.EntriesSlice() == nil || len(empty.Inverse().EntriesSlice()) != 0 {
		t.Fatal("empty bimap EntriesSlice must be empty and non-nil")
	}
}

// TestHashLookupsDoNotAllocate pins the zero-allocation lookup and iteration
// paths.
func TestHashLookupsDoNotAllocate(t *testing.T) {
	keys := []string{"alpha", "beta", "gamma", "delta"}
	set := NewSet(keys)
	funcSet := NewSetFunc(intEqual, weakHash, []int{3, 11, 19, 27})
	mapEntries := []Entry[string, int]{{Key: "alpha", Value: 1}, {Key: "beta", Value: 2}}
	value, _ := NewMap(mapEntries)
	multiset := NewMultiset(keys)
	bi, _ := NewBiMap([]bimap.Association[string, int]{{Key: "alpha", Value: 1}})
	mm := NewMultimap([]multimap.Association[string, int]{{Key: "alpha", Value: 1}})
	list := NewList([]int{1, 2, 3, 4})
	sink := 0
	allocs := testing.AllocsPerRun(200, func() {
		if set.Contains("gamma") && funcSet.Contains(19) && !funcSet.Contains(35) && multiset.Count("beta") == 1 {
			sink++
		}
		if got, found := value.Get("beta"); found {
			sink += got
		}
		if got, found := bi.GetValue("alpha"); found {
			sink += got
		}
		if got, found := bi.GetKey(1); found {
			sink += len(got)
		}
		if mm.ContainsKey("alpha") && mm.ContainsEntry("alpha", 1) {
			sink++
		}
	})
	if allocs != 0 {
		t.Fatalf("lookups allocate %.1f times per run", allocs)
	}
	allocs = testing.AllocsPerRun(200, func() {
		for item := range list.All() {
			sink += item
		}
		for item := range list.Reverse() {
			sink += item
		}
		for item := range set.All() {
			sink += len(item)
		}
	})
	if allocs != 0 {
		t.Fatalf("iteration allocates %.1f times per run", allocs)
	}
	if sink == 0 {
		t.Fatal("sink unused")
	}
}
