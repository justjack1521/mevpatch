package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var (
	errInvalidVersionFormat = func(str string) error {
		return fmt.Errorf("invalid version format: %s", str)
	}
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) Zero() bool {
	return v == Version{}
}

// NewVersion converts a semantic string representation of a version to a Version struct
func NewVersion(str string) (Version, error) {

	var parts = strings.Split(str, ".")
	if len(parts) != 3 {
		return Version{}, errInvalidVersionFormat(str)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, err
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, err
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, err
	}

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil

}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func GetChecksumForPath(path string) (string, error) {

	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()

	var hasher = sha256.New()
	if _, err := io.Copy(hasher, handle); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil

}

func GetChecksumForBytes(bytes []byte) string {
	var hasher = sha256.New()
	hasher.Write(bytes)
	return hex.EncodeToString(hasher.Sum(nil))
}
