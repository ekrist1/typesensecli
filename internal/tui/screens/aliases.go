package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

// NewAliases returns a Resource screen configured for Typesense collection
// aliases. Alias creation and updates use PUT /aliases/{alias}, so the generic
// resource screen prompts for a name first and then edits the request body.
func NewAliases(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:          "Aliases",
		Template:         templates.Alias,
		List:             client.ListAliases,
		Create:           nil,
		Update:           client.UpsertAlias,
		Delete:           client.DeleteAlias,
		CreateNamePrompt: true,
		ExtractItems:     aliasesExtractItems,
		ExtractDetail:    aliasesExtractDetail,
	}, w, h)
}

func aliasesExtractItems(body []byte) ([]string, error) {
	aliases, err := decodeAliases(body)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if name, ok := alias["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

func aliasesExtractDetail(body []byte, name string) []byte {
	aliases, err := decodeAliases(body)
	if err != nil {
		return nil
	}
	for _, alias := range aliases {
		if alias["name"] == name {
			b, _ := json.MarshalIndent(alias, "", "  ")
			return b
		}
	}
	return nil
}

func decodeAliases(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	var wrapped struct {
		Aliases []map[string]any `json:"aliases"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Aliases, nil
}
