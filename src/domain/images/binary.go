package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	releaseAsset = "station-linux-arm64"
	releaseBase  = "https://github.com/jterrazz/studio-station/releases/download/"
)

var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func (s *Service) needsBinary(given string) error {
	if given == "" && !releaseTag.MatchString(s.o.Version) {
		return &NoLinuxBinaryError{}
	}
	return nil
}

func (s *Service) binary(ctx context.Context, given string) (string, error) {
	if given != "" {
		if _, err := s.h.FS.Stat(given); err != nil {
			return "", &BinaryNotFoundError{Path: given}
		}
		return given, nil
	}
	return s.download(ctx)
}

func (s *Service) download(ctx context.Context) (string, error) {
	cached := path.Join(s.o.Paths.Cache, releaseAsset+"-"+s.o.Version)
	if _, err := s.h.FS.Stat(cached); err == nil {
		return cached, nil
	}
	fail := func(cause error) error {
		return &BuildFailedError{Image: "linux", Step: "fetching " + releaseAsset + " " + s.o.Version, Cause: cause}
	}
	base := releaseBase + s.o.Version + "/"
	status, sums, err := s.h.Net.Get(ctx, base+"checksums.txt", nil)
	if err != nil || status != 200 {
		return "", fail(statusError(base+"checksums.txt", status, err))
	}
	want := checksumOf(string(sums), releaseAsset)
	if want == "" {
		return "", fail(fmt.Errorf("checksums.txt lists no %s", releaseAsset))
	}
	status, body, err := s.h.Net.Get(ctx, base+releaseAsset, nil)
	if err != nil || status != 200 {
		return "", fail(statusError(base+releaseAsset, status, err))
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return "", fail(fmt.Errorf("checksum mismatch: checksums.txt says %s, the download is %s", want, got))
	}
	if err := s.h.FS.MkdirAll(s.o.Paths.Cache, 0o700); err != nil {
		return "", fail(err)
	}
	if err := s.h.FS.WriteFile(cached, body, 0o755); err != nil {
		return "", fail(err)
	}
	return cached, nil
}

func statusError(url string, status int, err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s answered %d", url, status)
}

func checksumOf(sums, asset string) string {
	for line := range strings.SplitSeq(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}
