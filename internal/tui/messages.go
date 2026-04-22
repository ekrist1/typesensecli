package tui

// APISuccessMsg is emitted when an API call returns a 2xx status.
type APISuccessMsg struct {
	Tag    string // screen-provided identifier so the right screen reacts
	Status int
	Body   []byte
}

// APIErrorMsg is emitted on non-2xx status or transport failure.
type APIErrorMsg struct {
	Tag    string
	Status int    // 0 if transport failure
	Body   []byte // may be empty
	Err    error  // nil for HTTP status errors, non-nil for transport
}
