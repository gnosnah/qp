package qp

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

func applyScript(t *testing.T, script []byte) {
	t.Helper()
	tr := New()
	md := make(map[string]int)
	for i := 0; i+2 < len(script); {
		op := script[i] % 4
		n := int(script[i+1]%12) + 1
		i += 2
		if i+n > len(script) {
			break
		}
		key := bytes.Clone(script[i : i+n])
		i += n
		if len(key) == 0 {
			continue
		}
		switch op {
		case 0, 1:
			v := int(key[0])
			old, upd := upsert(t, tr, key, v)
			mOld, mHas := md[string(key)]
			if upd != mHas {
				t.Fatalf("Upsert(%q) isUpdate=%v want %v", key, upd, mHas)
			}
			if mHas && old != mOld {
				t.Fatalf("Upsert(%q) old=%v want %v", key, old, mOld)
			}
			md[string(key)] = v
		case 2:
			old, found := del(t, tr, key)
			mOld, mHas := md[string(key)]
			if found != mHas {
				t.Fatalf("Delete(%q) found=%v want %v", key, found, mHas)
			}
			if mHas && old != mOld {
				t.Fatalf("Delete(%q) old=%v want %v", key, old, mOld)
			}
			delete(md, string(key))
		default:
			_, _ = get(t, tr, key)
			_, _, _ = gle(t, tr, key)
		}
	}

	if tr.Size() != len(md) {
		t.Fatalf("Size=%d want %d", tr.Size(), len(md))
	}

	want := make([]string, 0, len(md))
	for k, v := range md {
		gv, ok := get(t, tr, []byte(k))
		if !ok || gv != v {
			t.Fatalf("Get(%q)=(%v,%v) want (%v,true)", k, gv, ok, v)
		}
		want = append(want, k)
	}
	sort.Strings(want)

	var got []string
	it := tr.Iterator()
	for {
		k, _, ok := it.Next()
		if !ok {
			break
		}
		got = append(got, string(k))
	}
	if len(got) != len(want) {
		t.Fatalf("iter count %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("iter[%d]=%q want %q", i, got[i], want[i])
		}
	}

	walked := tr.Walk(len(md)+1, nil)
	if len(walked) != len(want) {
		t.Fatalf("Walk count %d want %d", len(walked), len(want))
	}
	for i, k := range want {
		if string(walked[i].Key) != k {
			t.Fatalf("Walk[%d]=%q want %q", i, walked[i].Key, k)
		}
	}

	for _, probe := range want {
		gk, gv, exact := gle(t, tr, []byte(probe))
		if !exact || string(gk) != probe || gv != md[probe] {
			t.Fatalf("GetLessOrEqual(%q)=(%q,%v,%v)", probe, gk, gv, exact)
		}
	}
	if len(want) > 0 {
		for _, probe := range []string{"\x00", "\xff", "m", want[0] + "\x00"} {
			gk, gv, exact := gle(t, tr, []byte(probe))
			wk, wv, wexact, wok := modelLessOrEqual(want, md, probe)
			if wok != (gk != nil) || (wok && (string(gk) != wk || gv != wv || exact != wexact)) {
				t.Fatalf("GetLessOrEqual(%q)=(%q,%v,%v) want (%q,%v,%v ok=%v)", probe, gk, gv, exact, wk, wv, wexact, wok)
			}
		}
	}
}

func modelLessOrEqual(want []string, md map[string]int, key string) (k string, v int, exact, ok bool) {
	idx := sort.SearchStrings(want, key)
	if idx < len(want) && want[idx] == key {
		return key, md[key], true, true
	}
	if idx == 0 {
		return "", 0, false, false
	}
	k = want[idx-1]
	return k, md[k], false, true
}

func Test_RandomModel(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		r := rand.New(rand.NewSource(seed))
		script := make([]byte, 800)
		_, _ = r.Read(script)
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			applyScript(t, script)
		})
	}
}

func FuzzTrieOps(f *testing.F) {
	f.Add([]byte{0, 1, 'a', 2, 1, 'a', 1, 2, 'a', 'b'})
	f.Fuzz(func(t *testing.T, script []byte) {
		applyScript(t, script)
	})
}
