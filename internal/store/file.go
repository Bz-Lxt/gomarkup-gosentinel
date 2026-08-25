package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"gosentinel/internal/rule"
	"gosentinel/internal/timeutil"
)

type FileStore struct {
	path string
	mu   sync.Mutex
	data Document
}

type Document struct {
	Version int64           `json:"version"`
	Rules   []rule.Snapshot `json:"rules"`
}

func Open(path string) (*FileStore, error) {
	s := &FileStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = Document{Version: 1, Rules: defaultRules()}
		return s.persistLocked()
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	if s.data.Rules == nil {
		s.data.Rules = []rule.Snapshot{}
	}
	return nil
}

func (s *FileStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileStore) Document() Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.data
	cp.Rules = append([]rule.Snapshot(nil), s.data.Rules...)
	return cp
}

func (s *FileStore) List() []rule.Snapshot {
	return s.Document().Rules
}

func (s *FileStore) Version() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Version
}

func (s *FileStore) Upsert(in rule.Snapshot) (rule.Snapshot, error) {
	in.Normalize()
	if err := rule.Validate(in); err != nil {
		return rule.Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Version++
	in.Version = s.data.Version
	in.UpdatedAt = timeutil.RFC3339(timeutil.Now())
	found := false
	for i, r := range s.data.Rules {
		if r.ID == in.ID || r.ResourceKey() == in.ResourceKey() {
			if in.ID == "" {
				in.ID = r.ID
			}
			s.data.Rules[i] = in
			found = true
			break
		}
	}
	if !found {
		if in.ID == "" {
			in.ID = newID(in.Service, in.Resource)
		}
		s.data.Rules = append(s.data.Rules, in)
	}
	if err := s.persistLocked(); err != nil {
		return rule.Snapshot{}, err
	}
	return in, nil
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.data.Rules[:0]
	found := false
	for _, r := range s.data.Rules {
		if r.ID == id {
			found = true
			continue
		}
		out = append(out, r)
	}
	if !found {
		return os.ErrNotExist
	}
	s.data.Rules = out
	s.data.Version++
	return s.persistLocked()
}

func (s *FileStore) Get(id string) (rule.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Rules {
		if r.ID == id {
			return r, true
		}
	}
	return rule.Snapshot{}, false
}

func newID(service, resource string) string {
	return service + "-" + resource + "-" + timeutil.Now().Format("150405")
}

func defaultRules() []rule.Snapshot {
	mk := func(id, service, resource string, qps int64) rule.Snapshot {
		r := rule.Default()
		r.ID = id
		r.Service = service
		r.Resource = resource
		r.Method = "*"
		r.QPS = qps
		r.Version = 1
		r.UpdatedAt = timeutil.RFC3339(timeutil.Now())
		return r
	}
	return []rule.Snapshot{
		mk("rule-gin-work", "demo-gin", "/work", 80),
		mk("rule-grpc-work", "demo-grpc", "/demo.Demo/Work", 80),
	}
}
