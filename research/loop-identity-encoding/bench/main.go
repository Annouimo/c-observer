// Command bench measures identifier encoding and matching costs on a real
// corpus. It intentionally uses only the standard library and does not import
// the production model: the goal is to compare encodings under identical data.
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	sink       uint64
	sinkString string
)

type parts struct{ owner, middle, path string }

func splitID(id string) parts {
	first := strings.IndexByte(id, ':')
	if first < 0 {
		return parts{id, "", ""}
	}
	rest := id[first+1:]
	second := strings.IndexByte(rest, ':')
	if second < 0 {
		return parts{id[:first], rest, ""}
	}
	return parts{id[:first], rest[:second], rest[second+1:]}
}

func load(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var r *bufio.Scanner
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			panic(err)
		}
		defer gz.Close()
		r = bufio.NewScanner(gz)
	} else {
		r = bufio.NewScanner(f)
	}
	var ids []string
	for r.Scan() {
		ids = append(ids, r.Text())
	}
	if err := r.Err(); err != nil {
		panic(err)
	}
	return ids
}

func heapDelta(build func() any) uint64 {
	runtime.GC()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	keep := build()
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(keep)
	if after.HeapAlloc < before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}

func main() {
	idsPath := flag.String("ids", "", "gzip or plain text file, one identifier per line")
	repeats := flag.Int("repeats", 1, "number of independent benchmark runs")
	flag.Parse()
	if *idsPath == "" {
		fmt.Fprintln(os.Stderr, "usage: bench -ids corpus.ids.txt.gz")
		os.Exit(2)
	}
	ids := load(*idsPath)
	if len(ids) == 0 {
		panic("empty corpus")
	}
	unique := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	fmt.Printf("corpus=%s occurrences=%d unique=%d avg_len=%.1f\n",
		*idsPath, len(ids), len(unique), float64(byteLen(ids))/float64(len(ids)))

	// Retained heap for each representation, measured independently.
	fmt.Printf("heap strings=%.1f KiB uint32=%.1f KiB hash64=%.1f KiB\n",
		float64(heapDelta(func() any {
			out := make([]string, len(ids))
			for i, id := range ids {
				out[i] = strings.Clone(id)
			}
			return out
		}))/1024,
		float64(heapDelta(func() any {
			intern := make(map[string]uint32, len(unique))
			out := make([]uint32, len(ids))
			for i, id := range ids {
				n, ok := intern[id]
				if !ok {
					n = uint32(len(intern) + 1)
					intern[id] = n
				}
				out[i] = n
			}
			return struct {
				intern map[string]uint32
				out    []uint32
			}{intern, out}
		}))/1024,
		float64(heapDelta(func() any {
			out := make([]uint64, len(ids))
			for i, id := range ids {
				out[i] = hashID(id)
			}
			return out
		}))/1024)

	run := func(name string, fn func(*testing.B)) {
		ns := make([]float64, 0, *repeats)
		var bytes, allocs int64
		for r := 0; r < *repeats; r++ {
			res := testing.Benchmark(fn)
			ns = append(ns, float64(res.NsPerOp()))
			bytes, allocs = res.AllocedBytesPerOp(), res.AllocsPerOp()
		}
		sort.Float64s(ns)
		fmt.Printf("%-22s median %10.1f ns/op (min %.1f max %.1f) %8d B/op %6d allocs/op\n",
			name, ns[len(ns)/2], ns[0], ns[len(ns)-1], bytes, allocs)
	}

	run("match/string-map", func(b *testing.B) {
		m := make(map[string]int, len(unique))
		for i, id := range unique {
			m[id] = i
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				n += uint64(m[id])
			}
		}
		sink += n
	})
	run("match/uint32-map", func(b *testing.B) {
		intern := make(map[string]uint32, len(unique))
		for i, id := range unique {
			intern[id] = uint32(i + 1)
		}
		m := make(map[uint32]int, len(unique))
		for _, id := range unique {
			m[intern[id]] = len(m)
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				n += uint64(m[intern[id]])
			}
		}
		sink += n
	})
	run("match/hash64-map", func(b *testing.B) {
		m := make(map[uint64]int, len(unique))
		for i, id := range unique {
			m[hashID(id)] = i
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				n += uint64(m[hashID(id)])
			}
		}
		sink += n
	})
	run("match/struct-key-map", func(b *testing.B) {
		type key struct{ owner, middle, path string }
		m := make(map[key]int, len(unique))
		for i, id := range unique {
			p := splitID(id)
			m[key{p.owner, p.middle, p.path}] = i
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				p := splitID(id)
				n += uint64(m[key{p.owner, p.middle, p.path}])
			}
		}
		sink += n
	})
	run("build/fmt-sprintf", func(b *testing.B) {
		ps := make([]parts, len(ids))
		for i, id := range ids {
			ps[i] = splitID(id)
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, p := range ps {
				sinkString = fmt.Sprintf("%s:%s:%s", p.owner, p.middle, p.path)
				n += uint64(len(sinkString))
			}
		}
		sink += n
	})
	run("build/string-concat", func(b *testing.B) {
		ps := make([]parts, len(ids))
		for i, id := range ids {
			ps[i] = splitID(id)
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, p := range ps {
				sinkString = p.owner + ":" + p.middle + ":" + p.path
				n += uint64(len(sinkString))
			}
		}
		sink += n
	})
	run("build/intern", func(b *testing.B) {
		ps := make([]parts, len(ids))
		for i, id := range ids {
			ps[i] = splitID(id)
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			intern := make(map[string]uint32, len(unique))
			for _, p := range ps {
				sinkString = p.owner + ":" + p.middle + ":" + p.path
				if _, ok := intern[sinkString]; !ok {
					intern[sinkString] = uint32(len(intern) + 1)
				}
				n += uint64(len(sinkString))
			}
		}
		sink += n
	})
	run("alloc/string-copy", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				sinkString = strings.Clone(id)
				n += uint64(len(sinkString))
			}
		}
		sink += n
	})
	run("lookup/uint32+reverse", func(b *testing.B) {
		reverse := make([]string, len(unique)+1)
		intern := make(map[string]uint32, len(unique))
		for i, id := range unique {
			intern[id] = uint32(i + 1)
			reverse[i+1] = id
		}
		b.ReportAllocs()
		b.ResetTimer()
		var n uint64
		for i := 0; i < b.N; i++ {
			for _, id := range ids {
				v := intern[id]
				n += uint64(len(reverse[v]))
			}
		}
		sink += n
	})
	_ = strconv.Itoa
}

func hashID(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
func byteLen(ss []string) int {
	n := 0
	for _, s := range ss {
		n += len(s)
	}
	return n
}
