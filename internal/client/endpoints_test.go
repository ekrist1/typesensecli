package client

import "testing"

func TestCollectionAndAliasEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "delete collection", wantM: "DELETE", wantP: "/collections/products", call: func() (string, string) { return DeleteCollection("products") }},
		{name: "list aliases", wantM: "GET", wantP: "/aliases", call: ListAliases},
		{name: "get alias", wantM: "GET", wantP: "/aliases/live-products", call: func() (string, string) { return GetAlias("live-products") }},
		{name: "upsert alias", wantM: "PUT", wantP: "/aliases/live-products", call: func() (string, string) { return UpsertAlias("live-products") }},
		{name: "delete alias", wantM: "DELETE", wantP: "/aliases/live-products", call: func() (string, string) { return DeleteAlias("live-products") }},
		{name: "list stopwords", wantM: "GET", wantP: "/stopwords", call: ListStopwords},
		{name: "get stopwords", wantM: "GET", wantP: "/stopwords/common", call: func() (string, string) { return GetStopwordSet("common") }},
		{name: "upsert stopwords", wantM: "PUT", wantP: "/stopwords/common", call: func() (string, string) { return UpsertStopwordSet("common") }},
		{name: "delete stopwords", wantM: "DELETE", wantP: "/stopwords/common", call: func() (string, string) { return DeleteStopwordSet("common") }},
		{name: "list presets", wantM: "GET", wantP: "/presets", call: ListPresets},
		{name: "get preset", wantM: "GET", wantP: "/presets/listing_view", call: func() (string, string) { return GetPreset("listing_view") }},
		{name: "upsert preset", wantM: "PUT", wantP: "/presets/listing_view", call: func() (string, string) { return UpsertPreset("listing_view") }},
		{name: "delete preset", wantM: "DELETE", wantP: "/presets/listing_view", call: func() (string, string) { return DeletePreset("listing_view") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}

func TestAPIKeyEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "list keys", wantM: "GET", wantP: "/keys", call: ListKeys},
		{name: "create key", wantM: "POST", wantP: "/keys", call: CreateKey},
		{name: "get key", wantM: "GET", wantP: "/keys/7", call: func() (string, string) { return GetKey("7") }},
		{name: "delete key", wantM: "DELETE", wantP: "/keys/7", call: func() (string, string) { return DeleteKey("7") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}

func TestAnalyticsEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "list rules", wantM: "GET", wantP: "/analytics/rules", call: ListAnalyticsRules},
		{name: "create rule", wantM: "POST", wantP: "/analytics/rules", call: UpsertAnalyticsRule},
		{name: "delete rule", wantM: "DELETE", wantP: "/analytics/rules/top_queries", call: func() (string, string) { return DeleteAnalyticsRule("top_queries") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}
