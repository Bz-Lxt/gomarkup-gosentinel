package rule

import "testing"

func TestValidateRejects(t *testing.T) {
	s := Default()
	s.Service = ""
	s.Resource = "x"
	s.QPS = 0
	err := Validate(s)
	if err == nil {
		t.Fatal("expected error")
	}
	ve := err.(*ValidationError)
	if len(ve.Details) < 2 {
		t.Fatalf("details %+v", ve.Details)
	}
}

func TestValidateOK(t *testing.T) {
	s := Default()
	s.Service = "demo-gin"
	s.Resource = "/work"
	if err := Validate(s); err != nil {
		t.Fatal(err)
	}
}
