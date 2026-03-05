package result_test

import (
	"encoding/json"
	"testing"

	"github.com/bernardoamc/chaind/internal/result"
)

func TestVerdict_String(t *testing.T) {
	tests := []struct {
		verdict result.Verdict
		want    string
	}{
		{result.VerdictConfirmedBase, "CONFIRMED_BASE"},
		{result.VerdictNotBase, "NOT_BASE"},
		{result.VerdictSameImage, "SAME_IMAGE"},
	}
	for _, tt := range tests {
		if got := tt.verdict.String(); got != tt.want {
			t.Errorf("Verdict(%d).String() = %q, want %q", int(tt.verdict), got, tt.want)
		}
	}
}

func TestVerdict_MarshalJSON(t *testing.T) {
	tests := []struct {
		verdict result.Verdict
		want    string
	}{
		{result.VerdictConfirmedBase, `"CONFIRMED_BASE"`},
		{result.VerdictNotBase, `"NOT_BASE"`},
		{result.VerdictSameImage, `"SAME_IMAGE"`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.verdict)
		if err != nil {
			t.Errorf("Marshal(%v): %v", tt.verdict, err)
			continue
		}
		if string(got) != tt.want {
			t.Errorf("Marshal(%v) = %s, want %s", tt.verdict, got, tt.want)
		}
	}
}

func TestVerdict_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input   string
		want    result.Verdict
		wantErr bool
	}{
		{`"CONFIRMED_BASE"`, result.VerdictConfirmedBase, false},
		{`"NOT_BASE"`, result.VerdictNotBase, false},
		{`"SAME_IMAGE"`, result.VerdictSameImage, false},
		{`"UNKNOWN_VERDICT"`, 0, true},
	}
	for _, tt := range tests {
		var got result.Verdict
		err := json.Unmarshal([]byte(tt.input), &got)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Unmarshal(%s): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("Unmarshal(%s): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Unmarshal(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
