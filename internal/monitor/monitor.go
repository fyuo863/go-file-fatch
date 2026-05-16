package monitor

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type ProgressTracker struct {
	downloaded atomic.Int64
	total      int64
}

// 传入完整文件大小比特数
func NewProgressTracker(total int64) *ProgressTracker {
	return &ProgressTracker{total: total}
}

func (p *ProgressTracker) Add(n int64) {
	p.downloaded.Add(n)
}

func (p *ProgressTracker) Run(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	startTime := time.Now()
	for {
		select {
		case <-ticker.C:
			downloaded := p.downloaded.Load()
			renderProgress(downloaded, p.total, time.Since(startTime))
			if downloaded >= p.total {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

func renderProgress(downloaded, total int64, elapsed time.Duration) {
	barWidth := 30
	pct := float64(downloaded) / float64(total)
	filled := int(pct * float64(barWidth))

	bar := strings.Repeat("=", filled)
	empty := strings.Repeat(" ", barWidth-filled)
	if filled > 0 && filled < barWidth {
		bar = bar[:filled-1] + ">"
	}

	speed := float64(downloaded) / elapsed.Seconds()

	fmt.Printf("\r[%s%s] %5.1f%%  %s / %s  %s/s  ",
		bar, empty, pct*100,
		formatSize(downloaded), formatSize(total), formatSize(int64(speed)))
	if downloaded >= total {
		fmt.Println()
	}
}
