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

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) Zero() bool {
	return v == Version{}
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// MinorBase returns the x.y.0 base for this version's minor series.
func (v Version) MinorBase() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: 0}
}

func NewVersion(str string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(str), ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format %q: expected major.minor.patch", str)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major in version %q: %w", str, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor in version %q: %w", str, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch in version %q: %w", str, err)
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

func GetChecksumForPath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func GetChecksumForBytes(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
