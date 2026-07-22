package config

import "testing"

func TestLoad(t *testing.T) {
	values := map[string]string{
		"WSP_TARGET_GROUP_JID": "120363000000000000@g.us",
		"WSP_CLASSIFIER_URL":   "http://127.0.0.1:8081/v1/classify",
	}
	config, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.SexualThreshold != 0.25 {
		t.Errorf("threshold = %v, want 0.25", config.SexualThreshold)
	}
	if !config.DeleteUncertain || !config.DeleteOnError {
		t.Error("safe defaults must delete uncertain content and classifier failures")
	}
	if config.MaxMediaBytes != 20<<20 {
		t.Errorf("max media bytes = %d, want %d", config.MaxMediaBytes, 20<<20)
	}
	if config.Workers != 1 {
		t.Errorf("workers = %d, want 1", config.Workers)
	}
}

func TestLoadWorkers(t *testing.T) {
	values := map[string]string{
		"WSP_TARGET_GROUP_JID": "120363000000000000@g.us",
		"WSP_CLASSIFIER_URL":   "http://classifier:8081/v1/classify",
		"WSP_WORKERS":          "2",
	}
	config, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Workers != 2 {
		t.Errorf("workers = %d, want 2", config.Workers)
	}

	values["WSP_WORKERS"] = "0"
	if _, err := Load(func(key string) string { return values[key] }); err == nil {
		t.Fatal("Load() error = nil, want invalid workers error")
	}
}

func TestLoadLogDecisions(t *testing.T) {
	values := map[string]string{
		"WSP_TARGET_GROUP_JID": "120363000000000000@g.us",
		"WSP_CLASSIFIER_URL":   "http://classifier:8081/v1/classify",
		"WSP_LOG_DECISIONS":    "true",
	}
	config, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !config.LogDecisions {
		t.Error("LogDecisions = false, se esperaba true")
	}

	values["WSP_LOG_DECISIONS"] = "no-es-booleano"
	if _, err := Load(func(key string) string { return values[key] }); err == nil {
		t.Fatal("Load() error = nil, se esperaba un error para WSP_LOG_DECISIONS")
	}
}

func TestLoadRejectsMissingOrNonGroupTarget(t *testing.T) {
	tests := []map[string]string{
		{"WSP_CLASSIFIER_URL": "http://localhost:8081/v1/classify"},
		{"WSP_TARGET_GROUP_JID": "56911111111@s.whatsapp.net", "WSP_CLASSIFIER_URL": "http://localhost:8081/v1/classify"},
		{"WSP_TARGET_GROUP_JID": "pending@g.us", "WSP_CLASSIFIER_URL": "http://localhost:8081/v1/classify"},
		{"WSP_TARGET_GROUP_JID": "120363000000000000@g.us"},
	}

	for _, values := range tests {
		if _, err := Load(func(key string) string { return values[key] }); err == nil {
			t.Errorf("Load(%v) error = nil, want validation error", values)
		}
	}
}
