package file

import "time"

type LocalFile struct {
	Path      string    `json:"Path"`
	Size      int64     `json:"Size"`
	Checksum  string    `json:"Checksum"`
	Timestamp time.Time `json:"Timestamp"`
}

func (f LocalFile) Zero() bool {
	return f == LocalFile{}
}
