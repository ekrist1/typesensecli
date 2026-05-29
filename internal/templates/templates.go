// Package templates exposes embedded JSON skeletons used by the TUI editor.
package templates

import _ "embed"

//go:embed nlmodel.json
var NLModel []byte

//go:embed curation.json
var Curation []byte

//go:embed synonym.json
var Synonym []byte

//go:embed stopword.json
var Stopword []byte

//go:embed preset.json
var Preset []byte

//go:embed alias.json
var Alias []byte

//go:embed conversation.json
var Conversation []byte

// All returns every template keyed by a short identifier.
func All() map[string][]byte {
	return map[string][]byte{
		"nlmodel":      NLModel,
		"curation":     Curation,
		"synonym":      Synonym,
		"stopword":     Stopword,
		"preset":       Preset,
		"alias":        Alias,
		"conversation": Conversation,
	}
}
