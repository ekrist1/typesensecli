package screens

import "testing"

func TestAnalyticsExtractItemsWrappedAndBare(t *testing.T) {
	wrapped := []byte(`{"rules":[{"name":"top","type":"popular_queries"},{"name":"misses","type":"nohits_queries"}]}`)
	ids, err := analyticsExtractItems(wrapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "top" || ids[1] != "misses" {
		t.Fatalf("got %v, want [top misses]", ids)
	}

	bare := []byte(`[{"name":"only","type":"counter"}]`)
	ids, err = analyticsExtractItems(bare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "only" {
		t.Fatalf("got %v, want [only]", ids)
	}
}

func TestAnalyticsExtractDetail(t *testing.T) {
	body := []byte(`{"rules":[{"name":"top","type":"popular_queries"}]}`)
	d := analyticsExtractDetail(body, "top")
	if d == nil {
		t.Fatal("expected detail for rule 'top'")
	}
}
