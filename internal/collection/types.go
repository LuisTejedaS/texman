package collection

// Request represents a single HTTP request in a collection.
type Request struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// Collection is a named group of requests loaded from a JSON file.
type Collection struct {
	Name     string    `json:"name"`
	Requests []Request `json:"requests"`
}
