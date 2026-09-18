package luau

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	code := `local a = 1 + 2`
	res, err := Parse(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, `"LocalStat"`) {
		t.Errorf("expected LocalStat, got: %s", res)
	}
	if !strings.Contains(res, `"Binary"`) {
		t.Errorf("expected Binary, got: %s", res)
	}
}

func TestParseCall(t *testing.T) {
	code := `print("halo roblox")`
	res, err := Parse(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, `"Call"`) || !strings.Contains(res, `"halo roblox"`) {
		t.Errorf("expected Call with string literal, got: %s", res)
	}
}

func TestParseError(t *testing.T) {
	code := `local a = + +`
	_, err := Parse(code)
	if err == nil {
		t.Fatalf("expected syntax error, got nil")
	}
}
