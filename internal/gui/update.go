package gui

type PatchUpdate interface {
}

type StatusUpdate struct {
	Primary   string
	Secondary string
}

type ProgressUpdate struct {
	ProgressUpdateType
	Reset bool
	Value float32
}

type ErrorUpdate struct {
	Value error
}

type ProgressUpdateType int

const (
	ProgressUpdateTypePrimary   ProgressUpdateType = iota
	ProgressUpdateTypeSecondary ProgressUpdateType = 1
)
