package downloader

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

//尝试中间件
//先使用HEAD(GET)来获取url的信息,使用中间件判断是否支持断点续传等功能
//若支持断点续传则

// FileMetadata 存储探测到的文件元数据
type FileMetadata struct {
	Url          string
	UserAgent    string
	FileName     string
	Size         int64
	AcceptRanges bool
}

type UrlInfo struct {
	Url       string
	UserAgent string
}

type MetadataFetcher interface {
	// Fetch 获取元数据，隐藏 HTTP 细节
	Fetch(ui UrlInfo) (*FileMetadata, error)
}

// 获取url信息
// HttpFetcher 是 MetadataFetcher 的一个实现
type HttpFetcher struct {
	Client *http.Client
}

func (f *HttpFetcher) Fetch(ui UrlInfo) (*FileMetadata, error) {
	// 1. 发起探测请求 (Range: bytes=0-0)
	req, _ := http.NewRequest("GET", ui.Url, nil)
	req.Header.Set("User-Agent", ui.UserAgent)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 2. 初始化元数据
	meta := &FileMetadata{
		Url:       ui.Url,
		UserAgent: ui.UserAgent,
		FileName:  getFileName(resp, ui.Url),
	}

	// 3. 判断是否支持断点续传 (检查 206 状态码)
	if resp.StatusCode == http.StatusPartialContent {
		meta.AcceptRanges = true
		// 解析 Content-Range: bytes 0-0/524288000
		contentRange := resp.Header.Get("Content-Range")
		if pos := strings.LastIndex(contentRange, "/"); pos != -1 {
			sizeStr := contentRange[pos+1:]
			size, _ := strconv.ParseInt(sizeStr, 10, 64)
			meta.Size = size
		}
	} else {
		meta.AcceptRanges = false
		meta.Size = resp.ContentLength
	}

	return meta, nil
}

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
