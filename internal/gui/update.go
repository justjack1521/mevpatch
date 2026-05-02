package gui

type PatchUpdate interface{}

type StatusUpdate struct {
	Primary   string
	Secondary string
}

type ProgressUpdate struct {
	ProgressUpdateType
	Reset bool
	Set   bool // if true, Value is an absolute 0-1 value rather than an increment
	Value float32
}

type ErrorUpdate struct {
	Value error
}

type LogUpdate struct {
	Value string
}

type ProgressUpdateType int

const (
	ProgressUpdateTypePrimary   ProgressUpdateType = iota
	ProgressUpdateTypeSecondary ProgressUpdateType = 1
)
