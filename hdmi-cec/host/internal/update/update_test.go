package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func server(t *testing.T, exe []byte, sum string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/o/r/releases", func(w http.ResponseWriter, _ *http.Request) {
		asset := func(tag, name string) string {
			return fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/dl/%s/%s"}`, name, srv.URL, tag, name)
		}
		release := func(tag string, draft, pre bool) string {
			return fmt.Sprintf(`{"tag_name":%q,"draft":%t,"prerelease":%t,"assets":[%s,%s]}`,
				tag, draft, pre, asset(tag, AssetName), asset(tag, SumsName))
		}
		fmt.Fprintf(w, "[%s,%s,%s,%s,%s]",
			release("hdmi-cec-v0.9.0", true, false), release("hdmi-cec-v0.8.1", false, true),
			release("other-v9.0.0", false, false), release("hdmi-cec-v0.10.0", false, false),
			release("hdmi-cec-v0.8.0", false, false))
	})
	mux.HandleFunc("/dl/hdmi-cec-v0.10.0/"+AssetName, func(w http.ResponseWriter, _ *http.Request) { w.Write(exe) })
	mux.HandleFunc("/dl/hdmi-cec-v0.10.0/"+SumsName, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  pico-cec.uf2\n%s  %s\n", sum, sum, AssetName)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func updater(srv *httptest.Server, current string) Updater {
	return Updater{Current: current, APIBase: srv.URL, Repo: "o/r", Client: srv.Client()}
}

func TestUpdateInstallsHighestPublishedRelease(t *testing.T) {
	data := []byte("new binary")
	sum := sha256.Sum256(data)
	srv := server(t, data, hex.EncodeToString(sum[:]))
	exe := filepath.Join(t.TempDir(), "hdmi-cec.exe")
	os.WriteFile(exe, []byte("old binary"), 0o755)

	release, updated, err := updater(srv, "0.7.0").Update(context.Background(), exe)
	if err != nil || !updated || release.Tag != "hdmi-cec-v0.10.0" {
		t.Fatalf("got %+v %v %v", release, updated, err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new binary" {
		t.Fatalf("exe = %q", got)
	}
	if got, _ := os.ReadFile(exe + ".old"); string(got) != "old binary" {
		t.Fatalf("old = %q", got)
	}
	Cleanup(exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatal("Cleanup left the old binary")
	}
}

func TestUpdateSkipsWhenCurrent(t *testing.T) {
	srv := server(t, nil, "")
	exe := filepath.Join(t.TempDir(), "hdmi-cec.exe")
	os.WriteFile(exe, []byte("current"), 0o755)
	if _, updated, err := updater(srv, "0.10.0").Update(context.Background(), exe); err != nil || updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if _, _, err := updater(srv, "dev").Update(context.Background(), exe); err == nil {
		t.Fatal("dev build must not update")
	}
}

func TestForceUpdateReinstallsLatest(t *testing.T) {
	data := []byte("new binary")
	sum := sha256.Sum256(data)
	srv := server(t, data, hex.EncodeToString(sum[:]))
	for _, current := range []string{"0.10.0", "0.11.0", "dev"} {
		exe := filepath.Join(t.TempDir(), "hdmi-cec.exe")
		os.WriteFile(exe, []byte("old binary"), 0o755)
		u := updater(srv, current)
		u.Force = true
		release, updated, err := u.Update(context.Background(), exe)
		if err != nil || !updated || release.Tag != "hdmi-cec-v0.10.0" {
			t.Fatalf("%s: got %+v %v %v", current, release, updated, err)
		}
		if got, _ := os.ReadFile(exe); string(got) != "new binary" {
			t.Fatalf("%s: exe = %q", current, got)
		}
	}
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	srv := server(t, []byte("tampered"), hex.EncodeToString(make([]byte, 32)))
	exe := filepath.Join(t.TempDir(), "hdmi-cec.exe")
	os.WriteFile(exe, []byte("old binary"), 0o755)
	if _, updated, err := updater(srv, "0.7.0").Update(context.Background(), exe); err == nil || updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Fatalf("exe changed to %q", got)
	}
}

func TestParseVersion(t *testing.T) {
	for _, bad := range []string{"dev", "0.7", "0.7.x", "v0.7.0", "0.7.0-rc1"} {
		if _, ok := ParseVersion(bad); ok {
			t.Errorf("accepted %q", bad)
		}
	}
	if v, ok := ParseVersion("hdmi-cec-v1.2.3"); !ok || v != [3]int{1, 2, 3} {
		t.Errorf("got %v %v", v, ok)
	}
}
