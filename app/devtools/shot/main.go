// shot:用无头 Chrome(DevTools 协议)按脚本截图。一个 Chrome 进程截完所有图。
//
//	go run ./devtools/shot -spec devtools/shots.json
//
// spec 是一个 JSON 数组,每项:
//
//	{"url": "...", "out": "docs/screenshots/x.png", "width": 1440, "height": 900, "scale": 2,
//	 "wait": "JS 表达式,返回 true 才截", "eval": "页面加载后先执行的 JS(可选)",
//	 "after": "条件满足后再执行的 JS(可选)", "settle_ms": 600}
//
// 只依赖标准库:WebSocket 客户端是手写的最小实现(只够和本机 Chrome 说话)。
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type shotSpec struct {
	URL         string  `json:"url"`
	Out         string  `json:"out"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Scale       float64 `json:"scale"`
	Wait        string  `json:"wait"`
	Eval        string  `json:"eval"`
	SettleMs    int     `json:"settle_ms"`
	Debug       string  `json:"debug"`       // 等不到条件时打印这个表达式的值,便于排查
	After       string  `json:"after"`       // 条件满足后、截图前执行的 JS(比如打开抽屉)
	Transparent bool    `json:"transparent"` // 透明背景(出图标用)
}

func main() {
	spec := flag.String("spec", "", "截图脚本 JSON")
	chrome := flag.String("chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "Chrome 路径")
	timeout := flag.Duration("timeout", 40*time.Second, "每张图最多等多久")
	flag.Parse()
	b, err := os.ReadFile(*spec)
	must(err)
	var shots []shotSpec
	must(json.Unmarshal(b, &shots))

	prof, _ := os.MkdirTemp("", "hz-shot-")
	defer os.RemoveAll(prof)
	cmd := exec.Command(*chrome, "--headless=new", "--remote-debugging-port=0", "--user-data-dir="+prof,
		"--no-first-run", "--no-default-browser-check", "--hide-scrollbars", "--disable-extensions",
		"--window-size=1440,900", "about:blank")
	stderr, _ := cmd.StderrPipe()
	must(cmd.Start())
	chromeCmd = cmd
	defer killChrome()

	wsRe := regexp.MustCompile(`DevTools listening on (ws://\S+)`)
	var browserWS string
	sc := bufio.NewScanner(stderr)
	deadline := time.Now().Add(20 * time.Second)
	for sc.Scan() && time.Now().Before(deadline) {
		if m := wsRe.FindStringSubmatch(sc.Text()); m != nil {
			browserWS = m[1]
			break
		}
	}
	if browserWS == "" {
		fail("Chrome 没有给出 DevTools 地址")
	}
	go io.Copy(io.Discard, stderr)
	u, _ := url.Parse(browserWS)

	var pageWS string
	for i := 0; i < 50 && pageWS == ""; i++ {
		resp, err := http.Get("http://" + u.Host + "/json/list")
		if err == nil {
			var ts []struct{ Type, WebSocketDebuggerUrl string }
			json.NewDecoder(resp.Body).Decode(&ts)
			resp.Body.Close()
			for _, t := range ts {
				if t.Type == "page" {
					pageWS = t.WebSocketDebuggerUrl
					break
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pageWS == "" {
		fail("找不到页面 target")
	}
	c, err := wsDial(pageWS)
	must(err)
	cdp := &client{ws: c}
	cdp.call("Page.enable", nil)

	for _, s := range shots {
		if s.Width == 0 {
			s.Width, s.Height = 1440, 900
		}
		if s.Scale == 0 {
			s.Scale = 2
		}
		cdp.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": s.Width, "height": s.Height, "deviceScaleFactor": s.Scale, "mobile": false})
		if s.Transparent {
			cdp.call("Emulation.setDefaultBackgroundColorOverride", map[string]any{"color": map[string]any{"r": 0, "g": 0, "b": 0, "a": 0}})
		} else {
			cdp.call("Emulation.setDefaultBackgroundColorOverride", map[string]any{})
		}
		cdp.call("Page.navigate", map[string]any{"url": s.URL})
		time.Sleep(300 * time.Millisecond)
		if s.Eval != "" {
			waitFor(cdp, "document.readyState === 'complete'", *timeout)
			cdp.eval(s.Eval)
		}
		if s.Wait != "" {
			if !waitFor(cdp, s.Wait, *timeout) {
				if s.Debug != "" {
					fmt.Fprintln(os.Stderr, "shot: debug =", cdp.eval(s.Debug))
				}
				fail("等不到条件:" + s.Wait + "(" + s.Out + ")")
			}
		}
		if s.After != "" {
			cdp.eval(s.After)
		}
		time.Sleep(time.Duration(max(s.SettleMs, 300)) * time.Millisecond)
		res := cdp.call("Page.captureScreenshot", map[string]any{"format": "png"})
		var r struct{ Data string }
		json.Unmarshal(res, &r)
		png, err := base64.StdEncoding.DecodeString(r.Data)
		must(err)
		os.MkdirAll(filepath.Dir(s.Out), 0o755)
		must(os.WriteFile(s.Out, png, 0o644))
		fmt.Printf("✓ %s (%d KB)\n", s.Out, len(png)/1024)
	}
}

func waitFor(c *client, expr string, timeout time.Duration) bool {
	end := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(end) {
		if last = c.eval(expr); last == "true" {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "shot: 最后一次求值 = %q\n", last)
	return false
}

type client struct {
	ws *wsConn
	id int
}

func (c *client) call(method string, params any) json.RawMessage {
	c.id++
	msg, _ := json.Marshal(map[string]any{"id": c.id, "method": method, "params": params})
	must(c.ws.send(msg))
	for {
		b, err := c.ws.recv()
		must(err)
		var r struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct{ Message string }
		}
		if json.Unmarshal(b, &r) == nil && r.ID == c.id {
			if r.Error != nil {
				fail(method + ": " + r.Error.Message)
			}
			return r.Result
		}
	}
}

func (c *client) eval(expr string) string {
	res := c.call("Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": true})
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	json.Unmarshal(res, &r)
	return strings.TrimSpace(string(r.Result.Value))
}

// ── 最小 WebSocket 客户端 ──

type wsConn struct {
	c net.Conn
	r *bufio.Reader
}

func wsDial(raw string) (*wsConn, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	rand.Read(key)
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		u.RequestURI(), u.Host, base64.StdEncoding.EncodeToString(key))
	r := bufio.NewReaderSize(conn, 1<<20)
	status, err := r.ReadString('\n')
	if err != nil || !strings.Contains(status, " 101 ") {
		return nil, fmt.Errorf("websocket 握手失败: %q %v", status, err)
	}
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if l == "\r\n" {
			break
		}
	}
	return &wsConn{conn, r}, nil
}

func (w *wsConn) send(p []byte) error {
	hdr := []byte{0x81}
	n := len(p)
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 1<<16:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	mask := make([]byte, 4)
	rand.Read(mask)
	hdr = append(hdr, mask...)
	body := make([]byte, n)
	for i := range p {
		body[i] = p[i] ^ mask[i%4]
	}
	_, err := w.c.Write(append(hdr, body...))
	return err
}

func (w *wsConn) recv() ([]byte, error) {
	var msg []byte
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(w.r, h); err != nil {
			return nil, err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0f
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			b := make([]byte, 2)
			io.ReadFull(w.r, b)
			n = uint64(binary.BigEndian.Uint16(b))
		case 127:
			b := make([]byte, 8)
			io.ReadFull(w.r, b)
			n = binary.BigEndian.Uint64(b)
		}
		if h[1]&0x80 != 0 {
			return nil, errors.New("服务器帧不应带掩码")
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(w.r, p); err != nil {
			return nil, err
		}
		switch op {
		case 0x8:
			return nil, errors.New("websocket closed")
		case 0x9, 0xA: // ping/pong,本地连接用不上
			continue
		}
		msg = append(msg, p...)
		if fin {
			return msg, nil
		}
	}
}

func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}

var chromeCmd *exec.Cmd

func killChrome() {
	if chromeCmd != nil && chromeCmd.Process != nil {
		_ = chromeCmd.Process.Kill()
		_ = chromeCmd.Wait()
	}
}

func fail(s string) {
	fmt.Fprintln(os.Stderr, "shot:", s)
	killChrome() // os.Exit 不跑 defer,这里必须手动关,否则留下孤儿 Chrome
	os.Exit(1)
}
