package screens

import "testing"

func TestStopwordsExtractItemsSupportsArrayAndWrappedResponses(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want []string
	}{
		{
			name: "array",
			body: []byte(`[{"id":"common","locale":"en","stopwords":["the"]}]`),
			want: []string{"common"},
		},
		{
			name: "wrapped",
			body: []byte(`{"stopwords":[{"id":"common"},{"name":"legacy"}]}`),
			want: []string{"common", "legacy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stopwordsExtractItems(tt.body)
			if err != nil {
				t.Fatalf("stopwordsExtractItems: %v", err)
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
