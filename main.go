package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const a = 10
const url = "http://localhost:8090/multimedia-exp1.zip"

var Resp struct {
	AcceptRanges  string
	ContentLength int
}

var wg sync.WaitGroup

func HEAD() *http.Response {
	// 创建一个 HEAD 请求
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		return nil
	}
	// 创建 HTTP 客户端
	client := &http.Client{}
	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		return nil
	}
	return resp
}

func GET(errCh chan error, file *os.File, start, end int) {
	// 创建一个 GET 请求
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		errCh <- err
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end)) // 请求从 start 字节到 end 字节的数据

	// 创建 HTTP 客户端
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		errCh <- err
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		// 处理错误
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
	if Resp.AcceptRanges == "" {
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
	errCh := make(chan error, a)

	resp := HEAD()

	if resp == nil {
		fmt.Println("HEAD 请求失败")
		return
	}
	defer resp.Body.Close()
	//预分配磁盘空间与文件占位
	f, err := os.Create("filename.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	ContentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		fmt.Println("转换 Content-Length 失败:", err)
		return
	}
	Resp.ContentLength = ContentLength
	Resp.AcceptRanges = resp.Header.Get("Accept-Ranges")
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
	DownloadManager(errCh, f, a)

	// 等待所有协程完成
	wg.Wait()
	close(errCh)
	// for i := 0; i < a; i++ {
	// 	<-c
	// }
	fmt.Println("下载完成")
	for n := range errCh {
		if n != nil {
			fmt.Println("下载过程中发生错误:", n)
		}
	}
}
