package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

// NewAnalytics returns a Resource screen for analytics rules. Both create and
// edit POST to /analytics/rules (POST upserts by the rule's "name" field), so
// the generic engine handles the full lifecycle without a wrapper.
func NewAnalytics(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:       "Analytics",
		Template:      templates.AnalyticsRule,
		List:          client.ListAnalyticsRules,
		Create:        client.UpsertAnalyticsRule,
		Update:        func(string) (string, string) { return client.UpsertAnalyticsRule() },
		Delete:        client.DeleteAnalyticsRule,
		ExtractItems:  analyticsExtractItems,
		ExtractDetail: analyticsExtractDetail,
	}, w, h)
}

func analyticsExtractItems(body []byte) ([]string, error) {
	rules, err := decodeAnalyticsRules(body)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		if name, ok := r["name"].(string); ok {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

func analyticsExtractDetail(body []byte, id string) []byte {
	rules, err := decodeAnalyticsRules(body)
	if err != nil {
		return nil
	}
	for _, r := range rules {
		if r["name"] == id {
			b, _ := json.MarshalIndent(r, "", "  ")
			return b
		}
	}
	return nil
}

func decodeAnalyticsRules(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj.Rules, nil
}
