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
