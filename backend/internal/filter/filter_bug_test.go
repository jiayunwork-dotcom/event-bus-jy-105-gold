package filter

import "testing"

func TestMatchRule_GteIncludesEqual(t *testing.T) {
	ok := matchRule(FilterRule{Field: "n", Operator: "gte", Value: 10}, map[string]interface{}{"n": 10})
	if !ok {
		t.Fatal("n=10 should match gte 10")
	}
}

func TestMatchRule_LteIncludesEqual(t *testing.T) {
	ok := matchRule(FilterRule{Field: "n", Operator: "lte", Value: 10}, map[string]interface{}{"n": 10})
	if !ok {
		t.Fatal("n=10 should match lte 10")
	}
}

func TestMatch_NilFilterMatchesAll(t *testing.T) {
	e := NewFilterEngine()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil filter panicked: %v", r)
		}
	}()
	if !e.Match(nil, map[string]interface{}{"n": 1}) {
		t.Fatal("nil filter should match all events")
	}
}

func TestMatchRule_BadRegexDoesNotMatch(t *testing.T) {
	ok := matchRule(FilterRule{Field: "s", Operator: "regex", Value: "("}, map[string]interface{}{"s": "hello"})
	if ok {
		t.Fatal("invalid regex should not match")
	}
}

func TestMatchRule_UnknownOperatorRejects(t *testing.T) {
	ok := matchRule(FilterRule{Field: "n", Operator: "bogus", Value: 1}, map[string]interface{}{"n": 1})
	if ok {
		t.Fatal("unknown operator should not match")
	}
}
