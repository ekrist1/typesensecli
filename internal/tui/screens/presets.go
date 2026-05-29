package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

func NewPresets(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:          "Presets",
		Template:         templates.Preset,
		List:             client.ListPresets,
		Create:           nil,
		Update:           client.UpsertPreset,
		Delete:           client.DeletePreset,
		CreateNamePrompt: true,
		ExtractItems:     presetsExtractItems,
		ExtractDetail:    presetsExtractDetail,
	}, w, h)
}

func presetsExtractItems(body []byte) ([]string, error) {
	presets, err := decodePresets(body)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(presets))
	for _, preset := range presets {
		if name, ok := preset["name"].(string); ok {
			names = append(names, name)
			continue
		}
		if id, ok := preset["id"].(string); ok {
			names = append(names, id)
		}
	}
	return names, nil
}

func presetsExtractDetail(body []byte, name string) []byte {
	presets, err := decodePresets(body)
	if err != nil {
		return nil
	}
	for _, preset := range presets {
		if preset["name"] == name || preset["id"] == name {
			b, _ := json.MarshalIndent(preset, "", "  ")
			return b
		}
	}
	return nil
}

func decodePresets(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	if raw, ok := obj["presets"]; ok {
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}

	out := make([]map[string]any, 0, len(obj))
	for name, raw := range obj {
		var preset map[string]any
		if err := json.Unmarshal(raw, &preset); err != nil {
			preset = map[string]any{"value": string(raw)}
		}
		if _, ok := preset["name"]; !ok {
			preset["name"] = name
		}
		out = append(out, preset)
	}
	return out, nil
}
