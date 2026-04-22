package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

func NewCurations(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:          "Curations",
		Template:         templates.Curation,
		List:             client.ListCurationSets,
		Create:           nil, // upsert-by-name: name captured before editor opens
		Update:           client.UpsertCurationSet,
		Delete:           client.DeleteCurationSet,
		CreateNamePrompt: true,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if name, ok := m["name"].(string); ok {
					ids = append(ids, name)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, name string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["name"] == name {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
}
