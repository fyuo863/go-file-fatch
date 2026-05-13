package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	threadCount = 50
	// 建议换一个更稳定的链接测试，或者确保这个链接没过期
	//"https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe"
	//"http://localhost:8090/multimedia-exp1.zip"
	downloadUrl = "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe"
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var Resp struct {
	AcceptRanges  uint
	ContentLength int
}

var realFileName string

var wg sync.WaitGroup

func getFileName(resp *http.Response, rawUrl string) string {
	// 1. 尝试从 Header 获取
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		parts := strings.Split(contentDisposition, "filename=")
		if len(parts) > 1 {
			name := strings.Trim(parts[1], "\"")
			// Header 里的名称有时也需要 Unescape
			decodedName, err := url.QueryUnescape(name)
			if err == nil {
				return decodedName
			}
			return name
		}
	}

	// 2. 从 URL 路径提取并格式化
	// 先解析 URL 排除 Query 参数的干扰
	u, err := url.Parse(rawUrl)
	var rawFileName string
	if err == nil {
		rawFileName = path.Base(u.Path)
	} else {
		rawFileName = path.Base(rawUrl)
	}

	// 核心步骤：将 %20 转换为 空格
	// PathUnescape 会把 %20 转为空格，但不会把 + 转为空格（符合路径规则）
	decodedName, err := url.PathUnescape(rawFileName)
	if err != nil {
		return rawFileName // 转换失败则返回原名
	}

	if decodedName == "" || decodedName == "." {
		return "downloaded_file.bin"
	}

	return decodedName
}

func HEAD() (*http.Response, error) {
	req, _ := http.NewRequest("GET", downloadUrl, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 如果支持分块下载，状态码应该是 206 Partial Content
	if resp.StatusCode == http.StatusPartialContent {
		// 注意：此时 Content-Range 包含总大小，例如 "bytes 0-0/524288000"
		// 而 Content-Length 只是当前这一小块的大小 (通常是 1)
		_, err := strconv.Atoi(resp.Header.Get("Content-Length"))
		if err != nil {
			fmt.Println("转换 Content-Length 失败:", err)
			return nil, err
		}

		test := resp.Header.Get("Content-Range")
		pos := strings.LastIndex(test, "/")
		if pos != -1 {
			totalSizeStr := test[pos+1:] // 截取斜杠之后的部分
			fmt.Println("提取结果:", totalSizeStr)
			ContentLength, err := strconv.Atoi(totalSizeStr)
			if err != nil {
				fmt.Println("转换 Content-Length 失败:", err)
				return nil, err
			}
			Resp.ContentLength = ContentLength
		}
		fmt.Println("🌰:", test)

		Resp.AcceptRanges = http.StatusPartialContent
		// 从 contentRange 中解析出总大小...
	}
	realFileName = getFileName(resp, downloadUrl)
	return resp, err
}

func GET(errCh chan error, file *os.File, start, end int) {
	// 创建一个 GET 请求
	req, err := http.NewRequest(http.MethodGet, downloadUrl, nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		errCh <- err
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end)) // 请求从 start 字节到 end 字节的数据
	req.Header.Set("User-Agent", userAgent)

	// 创建 HTTP 客户端
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		errCh <- err
		return
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		// 处理错误
		errCh <- fmt.Errorf("error reading body: %w", err) // 加上逻辑
	}
	//fmt.Println("分片数据长度:", len(data))
	//fmt.Println("🍌", resp.Body)
	if resp.StatusCode != http.StatusPartialContent {
		fmt.Printf("请求分片失败: %d-%d, 状态码: %s\n", start, end, resp.Status)
		errCh <- fmt.Errorf("分片 %d-%d 下载失败，状态码: %s", start, end, resp.Status)
		return
	}
	// 模拟处理响应
	//fmt.Println("🍎", resp)
	//fmt.Printf("下载分片: %d-%d, 状态码: %s\n", start, end, resp.Status)
	_, err = file.WriteAt(data, int64(start))
	if err != nil {
		errCh <- fmt.Errorf("写入失败: %w", err)
		return
	}
}

func DownloadManager(errCh chan error, file *os.File, a int) {
	if Resp.AcceptRanges == 0 {
		fmt.Println("不支持断点续传，退化成普通下载")
		a = 1
	}
	bytes := Resp.ContentLength
	fmt.Println(a, bytes, bytes/a)
	for i := 0; i < a; i++ {
		start := i * bytes / a
		end := start + bytes/a - 1
		if i == a-1 { // 最后一个分片可能需要处理剩余的字节
			end = bytes - 1
		}
		// 避免闭包问题，传递参数到协程中
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			GET(errCh, file, start, end)
		}(start, end)
	}
}

func main() {
	// 创建一个 HEAD 请求
	errCh := make(chan error, threadCount)

	resp, err := HEAD()

	fmt.Println("🍍", Resp)

	if resp == nil {
		fmt.Println("HEAD 请求失败", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	//预分配磁盘空间与文件占位
	var tmpFileName = "file_is_downloading.tmp"
	f, err := os.Create(tmpFileName)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("resp:%T", Resp.ContentLength)
	// 预分配磁盘空间（非常重要：防止下载中途磁盘满，且减少文件碎片）
	err = f.Truncate(int64(Resp.ContentLength))
	if err != nil {
		log.Fatal("无法预分配空间:", err)
	}

	// 打印响应头和状态码
	fmt.Println("状态码:", resp.Status)
	fmt.Println("响应头:")
	for key, values := range resp.Header {
		fmt.Printf("%s: %s\n", key, values)
	}
	// 启动下载管理器
	start := time.Now()
	defer func() {
		fmt.Printf("总耗时: %s\n", time.Since(start))
	}()
	DownloadManager(errCh, f, threadCount)

	// 等待所有协程完成
	wg.Wait()
	close(errCh)

	for n := range errCh {
		if n != nil {
			fmt.Println("下载过程中发生错误:", n)
		}
	}
	_ = f.Close()
	err = os.Rename(tmpFileName, realFileName)
	if err != nil {
		fmt.Printf("重命名失败: %v\n", err)
		return
	}
	fmt.Println("下载完成")
}
