// main_test.go
package main

import (
	"net/http"
	"testing"
)

func TestGetFileName(t *testing.T) {
	// 模拟一个带有 Content-Disposition 的响应
	resp := &http.Response{
		Header: http.Header{
			"Content-Disposition": []string{`attachment; filename="docker-installer.exe"`},
		},
	}
	name := getFileName(resp, "https://example.com/Docker%20Desktop%20Installer.exe")
	if name != "docker-installer.exe" {
		t.Errorf("expected docker-installer.exe, got %s", name)
	}
}
