package file

import "time"

type LocalFile struct {
	Path      string
	Size      int64
	Checksum  string
	Timestamp time.Time
}
