package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"go-fatch/internal/monitor"
)

const defaultThreadCount = 5

var Wg sync.WaitGroup

// DownloadManager 启动多线程下载，返回错误通道和文件句柄
func (m *FileMetadata) DownloadManager() (chan error, *os.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	tmpFileName := m.FileName + ".tmp"
	if m.FileName == "" {
		tmpFileName = "file_is_downloading.tmp"
	}
	f, err := os.Create(tmpFileName)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	threadCount := defaultThreadCount
	if !m.AcceptRanges {
		threadCount = 1
	}

	bytes := int(m.Size)
	errCh := make(chan error, threadCount)

	m.tracker = monitor.NewProgressTracker(m.Size)
	//开启一个进度条协程
	go m.tracker.Run(ctx)

	for i := 0; i < threadCount; i++ {
		start := i * bytes / threadCount
		end := start + bytes/threadCount - 1
		if i == threadCount-1 {
			end = bytes - 1
		}
		Wg.Add(1)
		go func(start, end int) {
			defer Wg.Done()
			m.getChunk(ctx, errCh, f, start, end)
		}(start, end)
	}

	go func() {
		Wg.Wait()
		f.Close()
		if err := os.Rename(tmpFileName, m.FileName); err != nil {
			errCh <- fmt.Errorf("重命名失败: %w", err)
			fmt.Printf("重命名失败: 临时文件: %s, 目标文件: %s, 错误: %v\n", tmpFileName, m.FileName, err)
		}
		close(errCh)
		cancel()
	}()

	return errCh, f, nil
}

func (m *FileMetadata) getChunk(ctx context.Context, errCh chan<- error, file *os.File, start, end int) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Url, nil)
	if err != nil {
		errCh <- err
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", m.UserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return
		default:
			errCh <- err
			return
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		errCh <- fmt.Errorf("分片 %d-%d 下载失败，状态码: %s", start, end, resp.Status)
		return
	}

	buf := make([]byte, 32*1024)
	offset := int64(start)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.WriteAt(buf[:n], offset); writeErr != nil {
				errCh <- fmt.Errorf("写入失败: %w", writeErr)
				return
			}
			m.tracker.Add(int64(n))
			offset += int64(n)
		}
		if readErr != nil {
			if readErr != io.EOF {
				errCh <- fmt.Errorf("读取失败: %w", readErr)
			}
			break
		}
	}
}
