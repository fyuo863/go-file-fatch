# go-fatch

一个用 Go 编写的多线程文件下载器，支持断点续传、实时进度条和跨平台构建。

## 特性

- **多线程分片下载** — 将文件切分为多个分片并发下载，充分利用带宽（默认 5 线程）
- **断点续传检测** — 自动探测服务器是否支持 Range 请求，不支持时自动回退到单线程
- **实时进度条** — 终端内显示下载进度、速度和预估剩余时间
- **内存友好** — 使用 32KB 缓冲区边下载边写入磁盘，避免大文件占用过多内存
- **文件名自动解析** — 优先从 `Content-Disposition` 提取文件名，其次从 URL 路径提取
- **跨平台构建** — CI 自动构建 linux/darwin/windows 的 amd64/arm64 二进制文件

## 安装

### 从源码构建

```bash
git clone https://github.com/fyuo863/go-file-fatch.git
cd go-file-fatch
go build -o go-fatch ./cmd/go-fatch
```

### 从 Release 下载

前往 [Releases](https://github.com/fyuo863/go-file-fatch/releases) 页面下载对应平台的预编译二进制。

## 快速开始

```go
package main

import (
	"fmt"
	"net/http"

	"go-fatch/internal/downloader"
)

func main() {
	// 1. 构造下载信息
	ui := downloader.UrlInfo{
		Url:       "https://example.com/file.zip",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}

	// 2. 探测文件元数据（大小、文件名、是否支持断点续传）
	fetcher := &downloader.HttpFetcher{Client: &http.Client{}}
	meta, err := fetcher.Fetch(ui)
	if err != nil {
		fmt.Printf("探测失败: %v\n", err)
		return
	}

	// 3. 启动多线程下载
	errCh, file, err := meta.DownloadManager()
	if err != nil {
		fmt.Printf("下载失败: %v\n", err)
		return
	}
	for err := range errCh {
		fmt.Printf("下载错误: %v\n", err)
	}
	fmt.Printf("下载完成: %s\n", file.Name())
}
```

## 项目结构

```
.
├── cmd/go-fatch/main.go          # 入口示例
├── internal/
│   ├── downloader/
│   │   ├── utils.go              # 元数据探测（MetadataFetcher / HttpFetcher）
│   │   ├── manager.go            # 多线程下载管理（DownloadManager）
│   │   └── pause.go              # 暂停/恢复（预留）
│   └── monitor/
│       └── monitor.go            # 终端进度条渲染
├── .github/workflows/ci.yml      # CI/CD（lint → test → build → release）
├── go.mod
└── README.md
```

## 工作流程

1. **元数据探测** — 发起 `GET` 请求，带 `Range: bytes=0-0` 头，根据响应判断：
   - `206 Partial Content` → 支持断点续传，从 `Content-Range` 解析文件大小
   - `200 OK` → 不支持，使用 `Content-Length`，回退单线程
2. **分片分配** — 将文件按线程数均匀切分，每个 goroutine 负责一个字节范围
3. **并发下载** — 各 goroutine 通过 `Range` 请求下载分片，写入临时文件的对应偏移
4. **进度显示** — 独立 goroutine 每 200ms 刷新终端进度条
5. **完成收尾** — 所有分片完成后关闭文件、重命名 `.tmp` → 目标文件名

## 许可证

MIT License
