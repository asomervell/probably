package orchestrator

import "testing"

func TestParseModelSpec(t *testing.T) {
	tests := []struct {
		name         string
		spec         string
		wantErr      bool
		wantProvider Provider
		wantModel    string
	}{
		{name: "anthropic", spec: "anthropic/claude-3-5-sonnet-20241022", wantProvider: ProviderAnthropic, wantModel: "claude-3-5-sonnet-20241022"},
		{name: "xai", spec: "xai/grok-3", wantProvider: ProviderXAI, wantModel: "grok-3"},
		{name: "google", spec: "google/gemini-2.0-flash", wantProvider: ProviderGoogle, wantModel: "gemini-2.0-flash"},
		{name: "groq", spec: "groq/llama-3.3-70b", wantProvider: ProviderGroq, wantModel: "llama-3.3-70b"},
		{name: "empty", spec: "", wantErr: true},
		{name: "anthropic no model", spec: "anthropic/", wantProvider: ProviderAnthropic, wantModel: ""},
		{name: "unknown provider", spec: "unknown/x", wantErr: true},
		{name: "no slash", spec: "anthropic", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseModelSpec(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseModelSpec(%q) = nil err, want err", tt.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModelSpec(%q) unexpected err: %v", tt.spec, err)
			}
			if spec.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", spec.Provider, tt.wantProvider)
			}
			if spec.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", spec.Model, tt.wantModel)
			}
		})
	}
}
