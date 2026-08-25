package store

import (
	"path/filepath"
	"testing"

	"gosentinel/internal/rule"
)

func TestUpsertValidateAndVersion(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	bad := rule.Default()
	if _, err := st.Upsert(bad); err == nil {
		t.Fatal("empty service should fail")
	}
	in := rule.Default()
	in.Service = "svc"
	in.Resource = "res"
	in.QPS = 40
	got, err := st.Upsert(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version < 2 {
		t.Fatalf("version %d", got.Version)
	}
	st2, err := Open(filepath.Join(dir, "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range st2.List() {
		if r.Resource == "res" && r.QPS == 40 {
			found = true
		}
	}
	if !found {
		t.Fatal("persist failed")
	}
}
