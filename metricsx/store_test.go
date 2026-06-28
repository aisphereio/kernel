package metricsx

import "testing"

func TestStoreHasHelpers(t *testing.T) {
	s := newStore()
	if s.hasCounter("requests_total") {
		t.Fatal("new store should not have counter")
	}
	if err := s.setCounter("requests_total", nil); err != nil {
		t.Fatal(err)
	}
	if !s.hasCounter("requests_total") {
		t.Fatal("counter should be marked as registered")
	}
	if err := s.setCounter("requests_total", nil); err == nil {
		t.Fatal("duplicate counter should return an error")
	}
}
