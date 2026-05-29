package screens

import "testing"

func TestPresetsExtractItemsSupportsArrayAndWrappedResponses(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want []string
	}{
		{
			name: "array",
			body: []byte(`[{"name":"listing_view","value":{"q":"*"}}]`),
			want: []string{"listing_view"},
		},
		{
			name: "wrapped",
			body: []byte(`{"presets":[{"name":"listing_view"},{"id":"fallback_id"}]}`),
			want: []string{"listing_view", "fallback_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := presetsExtractItems(tt.body)
			if err != nil {
				t.Fatalf("presetsExtractItems: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
