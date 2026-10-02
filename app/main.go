// Humanizer 启动器:下载模型 → 拉起 llama-server → 托管网页并反代 /api/*。
//
// 网页(web/)和默认配置(config/default.json)都编译进二进制,发布时只需要
// 启动器 + llama.cpp 引擎两样东西。开发时可以用 --web-dir 直接读磁盘上的网页。
package main

import (
	"embed"
	"io/fs"
	"os"

	"github.com/sgaofen/humanizer/app/internal/launcher"
)

//go:embed web
var webFS embed.FS

//go:embed config/default.json
var defaultConfig []byte

// 构建时用 -ldflags "-X main.version=..." 覆盖。
var version = "dev"

func main() {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	os.Exit(launcher.Main(launcher.Options{
		Web:           web,
		DefaultConfig: defaultConfig,
		Version:       version,
		Args:          os.Args[1:],
	}))
}
