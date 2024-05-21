package data

type DATA struct {
	Key       string                 `json:"key"`
	Data      map[string]interface{} `json:"data"`
	Attribute []string               `json:"attribute"`
	Root      string                 `json:"root"`
	CallBack  string                 `json:"url"`
}
