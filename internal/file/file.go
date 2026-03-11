package file

import "time"

type LocalFile struct {
	Path      string    `json:"ID"`
	Size      int64     `json:"Path"`
	Checksum  string    `json:"Checksum"`
	Timestamp time.Time `json:"Size"`
}

func (f LocalFile) Zero() bool {
	return f == LocalFile{}
}
