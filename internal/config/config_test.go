package config

import "testing"

func TestMissingIsStableAndSeparated(t *testing.T) {
	cfg := Config{}
	want := []string{"VOLCENGINE_API_KEY"}
	got := cfg.Missing()
	if len(got) != len(want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Missing() = %v, want %v", got, want)
		}
	}

	cfg.Volc.APIKey = "configured"
	if len(cfg.ModelMissing()) != 0 {
		t.Fatalf("ModelMissing() = %v, want empty", cfg.ModelMissing())
	}
}
