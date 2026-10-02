package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fakeGGUF(n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(1)).Read(b)
	copy(b, "GGUF")
	return b
}

// 模拟 HF:/resolve/ 返回 302 + X-Linked-*,/cdn/ 支持 Range。
// dropAfter>0 时第一次请求传到这么多字节就断开。
func hfServer(t *testing.T, data []byte, dropAfter int, ignoreRange bool) (*httptest.Server, *atomic.Int32, *[]string) {
	sum := sha256.Sum256(data)
	var drops atomic.Int32
	var ranges []string
	mux := http.NewServeMux()
	mux.HandleFunc("/r/m/resolve/main/f.gguf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Linked-Size", strconv.Itoa(len(data)))
		w.Header().Set("X-Linked-Etag", `"`+hex.EncodeToString(sum[:])+`"`)
		http.Redirect(w, r, "/cdn/f.gguf?sig=1", http.StatusFound)
	})
	mux.HandleFunc("/cdn/f.gguf", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			ranges = append(ranges, r.Header.Get("Range"))
		}
		if ignoreRange {
			r.Header.Del("Range")
		}
		if dropAfter > 0 && drops.Add(1) == 1 && r.Method == http.MethodGet {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(200)
			w.Write(data[:dropAfter])
			if hj, ok := w.(http.Hijacker); ok {
				c, _, _ := hj.Hijack()
				c.Close()
			}
			return
		}
		http.ServeContent(w, r, "f.gguf", time.Time{}, bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &drops, &ranges
}

func TestDownloadResumeAfterDrop(t *testing.T) {
	data := fakeGGUF(3 << 20)
	srv, _, ranges := hfServer(t, data, 1<<20, false)
	dest := filepath.Join(t.TempDir(), "m", "f.gguf")
	os.MkdirAll(filepath.Dir(dest), 0o755)
	d := &Download{URL: srv.URL + "/r/m/resolve/main/f.gguf", Dest: dest}
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("内容不一致")
	}
	if len(*ranges) < 2 || !strings.HasPrefix((*ranges)[1], "bytes=") {
		t.Fatalf("断线后应该用 Range 续传,实际请求:%q", *ranges)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal(".part 应该已改名")
	}
}

func TestDownloadResumeFromExistingPart(t *testing.T) {
	data := fakeGGUF(2 << 20)
	srv, _, ranges := hfServer(t, data, 0, false)
	dest := filepath.Join(t.TempDir(), "f.gguf")
	os.WriteFile(dest+".part", data[:700000], 0o644)
	d := &Download{URL: srv.URL + "/r/m/resolve/main/f.gguf", Dest: dest}
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if (*ranges)[0] != "bytes=700000-" {
		t.Fatalf("应从 700000 续传,实际 %q", *ranges)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("内容不一致")
	}
}

func TestDownloadServerIgnoresRange(t *testing.T) {
	data := fakeGGUF(1 << 20)
	srv, _, _ := hfServer(t, data, 0, true)
	dest := filepath.Join(t.TempDir(), "f.gguf")
	os.WriteFile(dest+".part", data[:1000], 0o644)
	d := &Download{URL: srv.URL + "/r/m/resolve/main/f.gguf", Dest: dest}
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) {
		t.Fatal("服务器不支持 Range 时应从头下载且内容正确")
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	data := fakeGGUF(1 << 20)
	srv, _, _ := hfServer(t, data, 0, false)
	dest := filepath.Join(t.TempDir(), "f.gguf")
	d := &Download{URL: srv.URL + "/r/m/resolve/main/f.gguf", Dest: dest, SHA256: strings.Repeat("ab", 32)}
	err := d.Run(context.Background())
	var pe *PermanentError
	if !errors.As(err, &pe) || pe.Code != "checksum" {
		t.Fatalf("应报校验失败,实际 %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("校验失败应删除半截文件")
	}
}

func TestDownloadNotGGUF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>captive portal</html>"))
	}))
	defer srv.Close()
	d := &Download{URL: srv.URL + "/x.gguf", Dest: filepath.Join(t.TempDir(), "x.gguf")}
	err := d.Run(context.Background())
	var pe *PermanentError
	if !errors.As(err, &pe) || pe.Code != "not_gguf" {
		t.Fatalf("应识别出不是 GGUF,实际 %v", err)
	}
}

func TestDownload404(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	d := &Download{URL: srv.URL + "/x.gguf", Dest: filepath.Join(t.TempDir(), "x.gguf")}
	err := d.Run(context.Background())
	var pe *PermanentError
	if !errors.As(err, &pe) || pe.Code != "not_found" {
		t.Fatalf("应报 not_found,实际 %v", err)
	}
}

func TestDownloadPauseKeepsPart(t *testing.T) {
	data := fakeGGUF(4 << 20)
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		for i := 0; i < len(data); i += 64 << 10 {
			if _, err := w.Write(data[i : i+64<<10]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(20 * time.Millisecond)
		}
	})
	srv := httptest.NewServer(slow)
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.gguf")
	ctx, cancel := context.WithCancel(context.Background())
	d := &Download{URL: srv.URL + "/f.gguf", Dest: dest}
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	if err := d.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("暂停应返回 Canceled,实际 %v", err)
	}
	st, err := os.Stat(dest + ".part")
	if err != nil || st.Size() == 0 || st.Size() >= int64(len(data)) {
		t.Fatalf("暂停后应保留部分文件:%v %v", st, err)
	}
}
