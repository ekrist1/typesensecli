package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

// NewNLModels returns a Resource screen configured for NL search models.
// The list endpoint returns an array of model objects with an "id" field.
func NewNLModels(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:  "NL Models",
		Template: templates.NLModel,
		List:     client.ListNLModels,
		Create:   client.CreateNLModel,
		Update:   client.UpdateNLModel,
		Delete:   client.DeleteNLModel,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if id, ok := m["id"].(string); ok {
					ids = append(ids, id)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, id string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["id"] == id {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
}
