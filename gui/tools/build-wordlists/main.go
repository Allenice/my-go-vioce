// Command build-wordlists compiles pinned public word lists for offline lookup.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
)

type entry struct {
	term      string
	frequency int
	readings  string
}
type source struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	License      string `json:"license"`
	InputSHA256  string `json:"input_sha256"`
	OutputSHA256 string `json:"output_sha256"`
	InputRows    int    `json:"input_rows"`
	Terms        int    `json:"terms"`
	MinFrequency int    `json:"min_frequency"`
}

func compile(path, id, url string, minimum int, out string) source {
	data, err := os.ReadFile(path)
	must(err)
	sum := sha256.Sum256(data)
	expected := map[string]string{
		"base": "3043b77068e09c9904f27cad82f12b6ebe9dbdb5aeff3b25e45ab7f9c1122b55",
		"it":   "e8cd42c9f5559735fa3bfb91515cbe051f991efd4a803038a1d5c0da85af3526",
	}
	if hex.EncodeToString(sum[:]) != expected[id] {
		panic("input does not match the pinned source: " + id)
	}
	src := source{ID: id, URL: url, License: "MIT", InputSHA256: hex.EncodeToString(sum[:]), MinFrequency: minimum}
	entries := map[string]entry{}
	args := pinyin.NewArgs()
	args.Heteronym = true
	cache := map[rune]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		src.InputRows++
		if len(fields) < 2 {
			panic("invalid word list row")
		}
		frequency, err := strconv.Atoi(fields[1])
		must(err)
		term := fields[0]
		if frequency < minimum || utf8.RuneCountInString(term) < 2 || utf8.RuneCountInString(term) > 8 {
			continue
		}
		var readings []string
		valid := true
		for _, r := range term {
			if !unicode.Is(unicode.Han, r) {
				valid = false
				break
			}
			p, ok := cache[r]
			if !ok {
				ps := pinyin.SinglePinyin(r, args)
				slices.Sort(ps)
				ps = slices.Compact(ps)
				p = strings.Join(ps, "/")
				cache[r] = p
			}
			if p == "" {
				valid = false
				break
			}
			readings = append(readings, p)
		}
		if valid && frequency > entries[term].frequency {
			entries[term] = entry{term, frequency, strings.Join(readings, " ")}
		}
	}
	var sorted []entry
	for _, e := range entries {
		sorted = append(sorted, e)
	}
	slices.SortFunc(sorted, func(a, b entry) int { return strings.Compare(a.term, b.term) })
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	writer := bufio.NewWriter(zip)
	for _, e := range sorted {
		_, err := fmt.Fprintf(writer, "%s\t%d\t%s\n", e.term, e.frequency, e.readings)
		must(err)
	}
	must(writer.Flush())
	must(zip.Close())
	src.Terms = len(sorted)
	outputSum := sha256.Sum256(compressed.Bytes())
	src.OutputSHA256 = hex.EncodeToString(outputSum[:])
	must(os.WriteFile(filepath.Join(out, id+".tsv.gz"), compressed.Bytes(), 0644))
	fmt.Printf("%s: %d input rows -> %d terms, %d compressed bytes\n", id, src.InputRows, src.Terms, compressed.Len())
	return src
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	jieba := flag.String("jieba", "", "jieba.dict.utf8 from gojieba v1.4.7")
	it := flag.String("it", "", "THUOCL_IT.txt from pinned THUOCL revision")
	out := flag.String("out", "internal/wordlists/data", "output directory")
	flag.Parse()
	if *jieba == "" || *it == "" {
		flag.Usage()
		os.Exit(2)
	}
	must(os.MkdirAll(*out, 0755))
	sources := []source{
		compile(*jieba, "base", "https://github.com/yanyiwu/gojieba/blob/v1.4.7/deps/cppjieba/dict/jieba.dict.utf8", 10, *out),
		compile(*it, "it", "https://github.com/thunlp/THUOCL/blob/a30ce79d895d01ab5132a5c74c29703ff7efb4cc/data/THUOCL_IT.txt", 1, *out),
	}
	manifest := struct {
		Version  int      `json:"version"`
		Readings string   `json:"readings"`
		Sources  []source `json:"sources"`
	}{1, "go-pinyin v0.21.0; all character readings, no tones", sources}
	b, err := json.MarshalIndent(manifest, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(*out, "manifest.json"), append(b, '\n'), 0644))
}
