package utils

type Op string

const (
	Add     Op = "add"
	Remove  Op = "remove"
	Replace Op = "replace"
	Move    Op = "move"
	Copy    Op = "copy"
	Test    Op = "test"
)

type JsonPatch struct {
	Op    Op     `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}
