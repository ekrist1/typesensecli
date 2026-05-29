package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

func NewStopwords(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:          "Stopwords",
		Template:         templates.Stopword,
		List:             client.ListStopwords,
		Create:           nil,
		Update:           client.UpsertStopwordSet,
		Delete:           client.DeleteStopwordSet,
		CreateNamePrompt: true,
		ExtractItems:     stopwordsExtractItems,
		ExtractDetail:    stopwordsExtractDetail,
	}, w, h)
}

func stopwordsExtractItems(body []byte) ([]string, error) {
	sets, err := decodeStopwords(body)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(sets))
	for _, set := range sets {
		if id, ok := set["id"].(string); ok {
			ids = append(ids, id)
			continue
		}
		if name, ok := set["name"].(string); ok {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

func stopwordsExtractDetail(body []byte, id string) []byte {
	sets, err := decodeStopwords(body)
	if err != nil {
		return nil
	}
	for _, set := range sets {
		if set["id"] == id || set["name"] == id {
			b, _ := json.MarshalIndent(set, "", "  ")
			return b
		}
	}
	return nil
}

func decodeStopwords(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	if raw, ok := obj["stopwords"]; ok {
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}

	out := make([]map[string]any, 0, len(obj))
	for id, raw := range obj {
		var set map[string]any
		if err := json.Unmarshal(raw, &set); err != nil {
			set = map[string]any{"stopwords": string(raw)}
		}
		if _, ok := set["id"]; !ok {
			set["id"] = id
		}
		out = append(out, set)
	}
	return out, nil
}
