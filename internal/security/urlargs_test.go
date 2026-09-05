package security

import "testing"

func TestURLArgFilter_Disabled(t *testing.T) {
	f, err := NewURLArgFilter(false, "")
	if err != nil {
		t.Fatalf("NewURLArgFilter: %v", err)
	}
	if got := f.Filter([]string{"deploy"}); got != nil {
		t.Fatalf("Filter() with filter disabled = %v, want nil", got)
	}
}

func TestURLArgFilter_RejectsShellMetacharacters(t *testing.T) {
	f, err := NewURLArgFilter(true, "")
	if err != nil {
		t.Fatalf("NewURLArgFilter: %v", err)
	}
	dangerous := []string{"; rm -rf /", "$(id)", "`id`", "a && b", "a|b"}
	got := f.Filter(dangerous)
	if len(got) != 0 {
		t.Fatalf("Filter(dangerous) = %v, want none to pass", got)
	}
}

func TestURLArgFilter_AllowsSafeValuesUpToMax(t *testing.T) {
	f, err := NewURLArgFilter(true, "")
	if err != nil {
		t.Fatalf("NewURLArgFilter: %v", err)
	}
	values := make([]string, 0, MaxURLArgs+5)
	for i := 0; i < MaxURLArgs+5; i++ {
		values = append(values, "deploy")
	}
	got := f.Filter(values)
	if len(got) != MaxURLArgs {
		t.Fatalf("Filter() len = %d, want %d", len(got), MaxURLArgs)
	}
}
