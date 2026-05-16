package main

import (
	"fmt"
	"net/http"

	"go-fatch/internal/downloader"
)

func main() {
	ui := downloader.UrlInfo{
		//"https://example.com/file.zip"
		Url:       "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36 Edg/148.0.0.0",
	}

	var fetcher downloader.MetadataFetcher = &downloader.HttpFetcher{
		Client: &http.Client{},
	}

	meta, err := fetcher.Fetch(ui)
	if err != nil {
		fmt.Printf("Fetch error: %v\n", err)
		return
	}
	fmt.Println(meta)
	fmt.Printf("File: %s, Size: %d, AcceptRanges: %v\n", meta.FileName, meta.Size, meta.AcceptRanges)
	//获得了信息
	//调用manager分配下载任务
	meta.DownloadManager()
	//downloader.Wg.Wait()
	// err = os.Rename(tmpFileName, m.FileName)
	// if err != nil {
	// 	fmt.Printf("重命名失败: %v\n", err)
	// 	return errCh, f, nil // 返回错误通道和文件句柄，允许调用者处理后续清理
	// }
}
