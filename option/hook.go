package option

type HookOptions struct {
	PreStart  Hook `json:"preStart"`
	PostStart Hook `json:"postStart"`
	PreStop   Hook `json:"preStop"`
	PostStop  Hook `json:"postStop"`
}
type Hook struct {
	HandleError bool            `json:"ignoreError"`
	HTTP        []HTTPExecution `json:"http"`
	// Others: like tcp, websocket etc.
}
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type HTTPExecution struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Headers []Header `json:"headers"`
}
