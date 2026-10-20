// Package wordlists owns versioned, read-only public vocabulary data.
package wordlists

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Disabled flags make both packs enabled for new installations and old settings.
// Only these two packs are shipped; the config is copied when a session starts.
type Config struct {
	BaseDisabled bool `json:"base_disabled,omitempty"`
	ITDisabled   bool `json:"it_disabled,omitempty"`
}

func (c Config) Enabled(source string) bool {
	switch source {
	case "base":
		return !c.BaseDisabled
	case "it":
		return !c.ITDisabled
	}
	return false
}

type Source struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	License      string `json:"license"`
	InputSHA256  string `json:"input_sha256"`
	OutputSHA256 string `json:"output_sha256"`
	InputRows    int    `json:"input_rows"`
	Terms        int    `json:"terms"`
	MinFrequency int    `json:"min_frequency"`
}

// Entry has precompiled character readings, not guessed phrase-level readings.
// Rank is log-normalized within its source; it is never a confidence score.
type Entry struct {
	Term      string
	Source    string
	Frequency int
	Rank      int
	Readings  [][]string
}

//go:embed data/*.tsv.gz data/manifest.json
var data embed.FS

var catalog = sync.OnceValues(func() ([]Source, error) {
	b, err := data.ReadFile("data/manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Version int      `json:"version"`
		Sources []Source `json:"sources"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != 1 || len(manifest.Sources) != 2 {
		return nil, errors.New("wordlists: invalid manifest")
	}
	return manifest.Sources, nil
})

// Catalog reads only the small manifest; it does not inflate or index word lists.
func Catalog() []Source {
	sources, err := catalog()
	if err != nil {
		return nil
	}
	return append([]Source(nil), sources...)
}

// Load reads the embedded snapshot and validates it. Call from a background
// worker and retain its index; no files, network or user vocabulary are modified.
func Load() ([]Entry, error) {
	sources, err := catalog()
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, source := range sources {
		blob, err := data.ReadFile("data/" + source.ID + ".tsv.gz")
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(blob)
		if hex.EncodeToString(hash[:]) != source.OutputSHA256 {
			return nil, errors.New("wordlists: digest mismatch")
		}
		rows, err := decode(blob, source)
		if err != nil {
			return nil, err
		}
		entries = append(entries, rows...)
	}
	return entries, nil
}

func decode(blob []byte, source Source) ([]Entry, error) {
	reader, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	// Compiled inputs are under 10 MiB each. Bound the parser even if a bundled
	// asset is accidentally replaced with an unrelated compressed file.
	limit := &io.LimitedReader{R: reader, N: 16*1024*1024 + 1}
	scanner := bufio.NewScanner(limit)
	var entries []Entry
	maxFrequency := 0
	previous := ""
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), "\t")
		if len(f) != 3 {
			return nil, errors.New("wordlists: invalid row")
		}
		frequency, err := strconv.Atoi(f[1])
		if err != nil || frequency < source.MinFrequency || frequency > 1_000_000_000 {
			return nil, errors.New("wordlists: invalid frequency")
		}
		term := f[0]
		n := utf8.RuneCountInString(term)
		if !utf8.ValidString(term) || n < 2 || n > 8 || term <= previous {
			return nil, errors.New("wordlists: invalid or unsorted term")
		}
		for _, r := range term {
			if !unicode.Is(unicode.Han, r) {
				return nil, errors.New("wordlists: non-Han term")
			}
		}
		entry := Entry{Term: term, Source: source.ID, Frequency: frequency}
		for _, syllable := range strings.Fields(f[2]) {
			readings := strings.Split(syllable, "/")
			for _, reading := range readings {
				if reading == "" || len(reading) > 8 {
					return nil, errors.New("wordlists: invalid reading")
				}
				for _, r := range reading {
					if r < 'a' || r > 'z' {
						return nil, errors.New("wordlists: invalid reading")
					}
				}
			}
			entry.Readings = append(entry.Readings, readings)
		}
		if len(entry.Readings) != n {
			return nil, errors.New("wordlists: reading count mismatch")
		}
		entries = append(entries, entry)
		previous = term
		maxFrequency = max(maxFrequency, frequency)
		if len(entries) > source.Terms {
			return nil, errors.New("wordlists: excessive entries")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit.N <= 0 || len(entries) != source.Terms {
		return nil, fmt.Errorf("wordlists: count mismatch for %s", source.ID)
	}
	for i := range entries {
		entries[i].Rank = int(math.Round(100 * math.Log1p(float64(entries[i].Frequency)) / math.Log1p(float64(maxFrequency))))
	}
	return entries, nil
}
