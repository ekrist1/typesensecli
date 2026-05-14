package client

import (
	"fmt"
	"net/url"
)

// Each helper returns (method, path) for a Typesense endpoint.
// Paths verified against https://typesense.org/docs/30.2/api/.

func ListCollections() (string, string) { return "GET", "/collections" }
func GetCollection(name string) (string, string) {
	return "GET", fmt.Sprintf("/collections/%s", name)
}
func PatchCollection(name string) (string, string) {
	return "PATCH", fmt.Sprintf("/collections/%s", name)
}
func DeleteCollection(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/collections/%s", name)
}

func ListAliases() (string, string) { return "GET", "/aliases" }
func GetAlias(name string) (string, string) {
	return "GET", fmt.Sprintf("/aliases/%s", name)
}
func UpsertAlias(name string) (string, string) {
	return "PUT", fmt.Sprintf("/aliases/%s", name)
}
func DeleteAlias(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/aliases/%s", name)
}

func ListNLModels() (string, string)  { return "GET", "/nl_search_models" }
func CreateNLModel() (string, string) { return "POST", "/nl_search_models" }
func UpdateNLModel(id string) (string, string) {
	return "PUT", fmt.Sprintf("/nl_search_models/%s", id)
}
func DeleteNLModel(id string) (string, string) {
	return "DELETE", fmt.Sprintf("/nl_search_models/%s", id)
}

func ListCurationSets() (string, string) { return "GET", "/curation_sets" }
func GetCurationSet(name string) (string, string) {
	return "GET", fmt.Sprintf("/curation_sets/%s", name)
}
func UpsertCurationSet(name string) (string, string) {
	return "PUT", fmt.Sprintf("/curation_sets/%s", name)
}
func DeleteCurationSet(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/curation_sets/%s", name)
}

func ListSynonymSets() (string, string) { return "GET", "/synonym_sets" }
func GetSynonymSet(name string) (string, string) {
	return "GET", fmt.Sprintf("/synonym_sets/%s", name)
}
func UpsertSynonymSet(name string) (string, string) {
	return "PUT", fmt.Sprintf("/synonym_sets/%s", name)
}
func DeleteSynonymSet(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/synonym_sets/%s", name)
}

func ListConversationModels() (string, string)  { return "GET", "/conversations/models" }
func CreateConversationModel() (string, string) { return "POST", "/conversations/models" }
func UpdateConversationModel(id string) (string, string) {
	return "PUT", fmt.Sprintf("/conversations/models/%s", id)
}
func DeleteConversationModel(id string) (string, string) {
	return "DELETE", fmt.Sprintf("/conversations/models/%s", id)
}

// ConversationTest performs a search with a conversation model attached.
// Path and body shape verified against the Typesense Conversational Search docs.
func ConversationTest() (string, string) { return "POST", "/multi_search" }

// SearchDocuments issues a text search against a single collection. The caller
// supplies the query text and a comma-separated list of fields to search.
func SearchDocuments(collection, q, queryBy string) (string, string) {
	v := url.Values{}
	v.Set("q", q)
	v.Set("query_by", queryBy)
	v.Set("per_page", "25")
	return "GET", fmt.Sprintf("/collections/%s/documents/search?%s", url.PathEscape(collection), v.Encode())
}
