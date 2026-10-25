// Package update replaces the running hdmi-cec.exe with the newest GitHub release.
package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	Repo      = "alexander-litvinovich/entertainment-center"
	TagPrefix = "hdmi-cec-v"
	AssetName = "hdmi-cec.exe"
	SumsName  = "SHA256SUMS"
	maxAsset  = 64 << 20
)

type Release struct {
	Tag     string
	Version string
	exeURL  string
	sumsURL string
}

type Updater struct {
	Current string // version of the running binary, e.g. "0.7.0"
	APIBase string // https://api.github.com unless testing
	Repo    string
	Client  *http.Client
}

func New(current string) Updater {
	return Updater{Current: current, APIBase: "https://api.github.com", Repo: Repo,
		Client: &http.Client{Timeout: 5 * time.Minute}}
}

// ParseVersion accepts "0.7.0" or "hdmi-cec-v0.7.0".
func ParseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(strings.TrimPrefix(s, TagPrefix), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// Latest returns the highest published hdmi-cec release that carries the Windows binary.
func (u Updater) Latest(ctx context.Context) (Release, error) {
	var releases []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	body, err := u.get(ctx, u.APIBase+"/repos/"+u.Repo+"/releases?per_page=30", 4<<20)
	if err != nil {
		return Release{}, err
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return Release{}, fmt.Errorf("parse releases: %w", err)
	}
	var best Release
	var bestVersion [3]int
	for _, r := range releases {
		v, ok := ParseVersion(r.Tag)
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.Tag, TagPrefix) || !ok {
			continue
		}
		candidate := Release{Tag: r.Tag, Version: strings.TrimPrefix(r.Tag, TagPrefix)}
		for _, a := range r.Assets {
			switch a.Name {
			case AssetName:
				candidate.exeURL = a.URL
			case SumsName:
				candidate.sumsURL = a.URL
			}
		}
		if candidate.exeURL != "" && candidate.sumsURL != "" && (best.Tag == "" || newer(v, bestVersion)) {
			best, bestVersion = candidate, v
		}
	}
	if best.Tag == "" {
		return Release{}, errors.New("no published hdmi-cec release with " + AssetName)
	}
	return best, nil
}

// Update installs the latest release over exe when it is newer than u.Current.
// It returns the release it found and whether exe was replaced.
func (u Updater) Update(ctx context.Context, exe string) (Release, bool, error) {
	current, ok := ParseVersion(u.Current)
	if !ok {
		return Release{}, false, fmt.Errorf("version %q is a development build; updates are disabled", u.Current)
	}
	release, err := u.Latest(ctx)
	if err != nil {
		return Release{}, false, err
	}
	latest, _ := ParseVersion(release.Version)
	if !newer(latest, current) {
		return release, false, nil
	}
	sums, err := u.get(ctx, release.sumsURL, 64<<10)
	if err != nil {
		return release, false, err
	}
	want, err := checksum(sums, AssetName)
	if err != nil {
		return release, false, err
	}
	data, err := u.get(ctx, release.exeURL, maxAsset)
	if err != nil {
		return release, false, err
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != want {
		return release, false, fmt.Errorf("%s checksum mismatch", AssetName)
	}
	if err := Install(exe, data); err != nil {
		return release, false, err
	}
	return release, true, nil
}

func checksum(sums []byte, name string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s has no entry for %s", SumsName, name)
}

func (u Updater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hdmi-cec/"+u.Current)
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s: response exceeds %d bytes", url, limit)
	}
	return data, nil
}

// Install swaps exe for data. Windows allows renaming a running executable, so the old
// file moves to exe+".old" (removed by Cleanup on the next start) and the new one takes its path.
func Install(exe string, data []byte) error {
	next, old := exe+".new", exe+".old"
	if err := os.WriteFile(next, data, 0o755); err != nil {
		return err
	}
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(next)
		return err
	}
	if err := os.Rename(next, exe); err != nil {
		_ = os.Rename(old, exe)
		_ = os.Remove(next)
		return err
	}
	return nil
}

// Cleanup removes the previous binary left by Install.
func Cleanup(exe string) { _ = os.Remove(exe + ".old") }
