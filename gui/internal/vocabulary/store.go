package vocabulary

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Term struct {
	Term           string    `json:"term"`
	NormalizedTerm string    `json:"normalizedTerm"`
	Source         string    `json:"source"`
	Weight         int       `json:"weight"`
	Locked         bool      `json:"locked"`
	CreatedAt      time.Time `json:"createdAt"`
	LastUsedAt     time.Time `json:"lastUsedAt"`
}
type Store struct {
	Terms []Term `json:"terms"`
}

func Normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func (s *Store) Put(t Term, ai bool) error {
	t.Term = strings.TrimSpace(t.Term)
	t.NormalizedTerm = Normalize(t.Term)
	if t.NormalizedTerm == "" || len(t.Term) > 256 || !utf8.ValidString(t.Term) || t.Weight < 0 || t.Weight > 100 {
		return errors.New("invalid vocabulary term")
	}
	switch t.Source {
	case "manual", "correction", "ai", "imported":
	default:
		return errors.New("invalid source")
	}
	for i, v := range s.Terms {
		if v.NormalizedTerm == t.NormalizedTerm {
			if ai && v.Locked {
				return errors.New("term is locked")
			}
			t.CreatedAt = v.CreatedAt
			t.LastUsedAt = v.LastUsedAt
			s.Terms[i] = t
			return nil
		}
	}
	if len(s.Terms) >= 2000 {
		return errors.New("vocabulary full")
	}
	t.CreatedAt = time.Now()
	s.Terms = append(s.Terms, t)
	return nil
}
func (s *Store) Delete(term string, ai bool) error {
	for i, v := range s.Terms {
		if v.NormalizedTerm == Normalize(term) {
			if ai && v.Locked {
				return errors.New("term is locked")
			}
			s.Terms = append(s.Terms[:i], s.Terms[i+1:]...)
			return nil
		}
	}
	return nil
}
func (s *Store) Top(n int) []string {
	n = max(0, min(n, 100))
	terms := append([]Term(nil), s.Terms...)
	sort.SliceStable(terms, func(i, j int) bool {
		if terms[i].Locked != terms[j].Locked {
			return terms[i].Locked
		}
		return terms[i].Weight > terms[j].Weight
	})
	out := []string{}
	for _, t := range terms {
		if len(out) >= n {
			break
		}
		out = append(out, t.Term)
	}
	now := time.Now()
	for i := range s.Terms {
		for _, t := range out {
			if t == s.Terms[i].Term {
				s.Terms[i].LastUsedAt = now
			}
		}
	}
	return out
}
func Load(path string) (Store, error) {
	var s Store
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if len(b) > 2*1024*1024 {
		return s, errors.New("vocabulary too large")
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return Store{}, e
	}
	validated := Store{}
	for _, t := range s.Terms {
		if e = validated.Put(t, false); e != nil {
			return Store{}, e
		}
		for i := range validated.Terms {
			if validated.Terms[i].NormalizedTerm == Normalize(t.Term) {
				if !t.CreatedAt.IsZero() {
					validated.Terms[i].CreatedAt = t.CreatedAt
				}
				validated.Terms[i].LastUsedAt = t.LastUsedAt
			}
		}
	}
	return validated, nil
}
func (s *Store) Save(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(filepath.Dir(path), ".vocabulary-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e != nil {
		tmp.Close()
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
