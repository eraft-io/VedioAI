package main

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// 视频文件缓存
type VideoCache struct {
	data     []byte
	mimeType string
	modTime  time.Time
	path     string
}

var videoCache = make(map[string]*VideoCache)
var cacheMutex sync.RWMutex

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "VideoSubtitle",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// 处理本地文件请求 /local-file/path/to/file
					if strings.HasPrefix(r.URL.Path, "/local-file/") {
						encodedPath := strings.TrimPrefix(r.URL.Path, "/local-file/")
						// URL 解码
						filePath, err := url.PathUnescape(encodedPath)
						if err != nil {
							filePath = encodedPath
						}

						// 处理 Windows 路径（如 C:/Users/... 或 /C:/Users/...）
						// 移除开头的斜杠（如果后面跟着盘符）
						if len(filePath) >= 3 && filePath[0] == '/' && filePath[2] == ':' {
							filePath = filePath[1:] // 移除开头的 /
						}
						// 将正斜杠转换为系统路径分隔符（Windows 需要）
						filePath = strings.ReplaceAll(filePath, "/", string(os.PathSeparator))

						// Unix 系统确保路径以 / 开头
						if os.PathSeparator == '/' && !strings.HasPrefix(filePath, "/") {
							filePath = "/" + filePath
						}

						println("请求视频文件:", filePath)

						// 检查缓存
						cacheMutex.RLock()
						cache, exists := videoCache[filePath]
						cacheMutex.RUnlock()

						if !exists {
							// 读取文件到内存
							println("读取文件到内存:", filePath)
							data, err := os.ReadFile(filePath)
							if err != nil {
								println("无法读取文件:", err.Error())
								http.NotFound(w, r)
								return
							}

							// 获取文件信息
							stat, err := os.Stat(filePath)
							if err != nil {
								println("无法获取文件信息:", err.Error())
								http.NotFound(w, r)
								return
							}

							// 根据文件扩展名设置 Content-Type
							ext := ""
							if idx := strings.LastIndex(filePath, "."); idx != -1 {
								ext = strings.ToLower(filePath[idx+1:])
							}
							contentType := "application/octet-stream"
							switch ext {
							case "mp4":
								contentType = "video/mp4"
							case "webm":
								contentType = "video/webm"
							case "ogg", "ogv":
								contentType = "video/ogg"
							case "mkv":
								contentType = "video/webm"
							case "mov":
								contentType = "video/quicktime"
							case "avi":
								contentType = "video/x-msvideo"
							}

							// 存入缓存
							cache = &VideoCache{
								data:     data,
								mimeType: contentType,
								modTime:  stat.ModTime(),
								path:     filePath,
							}
							cacheMutex.Lock()
							videoCache[filePath] = cache
							cacheMutex.Unlock()

							println("文件已缓存，大小:", len(data))
						} else {
							println("使用缓存:", filePath)
						}

						// 从内存提供文件
						w.Header().Set("Content-Type", cache.mimeType)
						w.Header().Set("Content-Length", fmt.Sprintf("%d", len(cache.data)))
						w.Header().Set("Accept-Ranges", "bytes")
						http.ServeContent(w, r, cache.path, cache.modTime, bytes.NewReader(cache.data))
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
