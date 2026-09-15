package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexDefaultsToNoTokenAndListsFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "照片", "旅行 photo.jpg"), "image")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	for _, expected := range []string{"旅行 photo.jpg", "照片", "5 B", "方序传文件", "方寸之间，传递有序"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("page does not contain %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), "?token=") {
		t.Error("default page contains a tokenized URL")
	}
	for _, hiddenOnMobile := range []string{"手机访问地址 · 扫码即可打开", "data:image/png", "共享目录："} {
		if strings.Contains(response.Body.String(), hiddenOnMobile) {
			t.Errorf("mobile page unexpectedly contains %q", hiddenOnMobile)
		}
	}
}

func TestIndexShowsServiceControlsAndSortSelectorToLocalPC(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "example.txt"), "example")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:43210"
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	for _, expected := range []string{"服务运行中", "停止传输服务", `action="/stop"`, `aria-label="当前排序方式"`, "时间：从新到旧", "时间：从旧到新", "名称：正序", "名称：倒序", `data-time=`, "共享目录：", "该目录下的文件可在其他设备访问和下载", `action="/directory"`, "选择文件夹…", "应用路径", "上传文件", "上传到电脑的共享目录", `name="files" multiple`, "开始上传"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("local page does not contain %q", expected)
		}
	}
	for _, removed := range []string{"<h2>共享文件</h2>", "默认按最近更新时间排列"} {
		if strings.Contains(response.Body.String(), removed) {
			t.Errorf("local page still contains removed copy %q", removed)
		}
	}
}

func TestLocalControlCanChangeDirectory(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	mustWrite(t, filepath.Join(oldRoot, "旧文件.txt"), "old")
	mustWrite(t, filepath.Join(newRoot, "新文件.txt"), "new")
	s, err := newServer(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)

	form := url.Values{"admin_token": {s.token}, "action": {"apply"}, "path": {newRoot}}
	request := httptest.NewRequest(http.MethodPost, "/directory", strings.NewReader(form.Encode()))
	request.RemoteAddr = "127.0.0.1:43210"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?status=changed" {
		t.Fatalf("change directory: status=%d location=%q", response.Code, response.Header().Get("Location"))
	}

	root, _ := s.roots()
	wantRoot, err := filepath.Abs(newRoot)
	if err != nil {
		t.Fatal(err)
	}
	if root != wantRoot {
		t.Fatalf("shared root %q, want %q", root, wantRoot)
	}
	pageRequest := httptest.NewRequest(http.MethodGet, "/?status=changed", nil)
	pageRequest.RemoteAddr = "127.0.0.1:43210"
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, pageRequest)
	for _, expected := range []string{"新文件.txt", "共享目录已更新。", `value="` + wantRoot + `"`} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Errorf("updated page does not contain %q", expected)
		}
	}
	if strings.Contains(page.Body.String(), "旧文件.txt") {
		t.Error("updated page still lists a file from the previous directory")
	}
}

func TestWebUploadAddsFilesWithoutOverwritingExistingFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "资料.txt"), "existing")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, content := range map[string]string{"资料.txt": "new", "照片.jpg": "image"} {
		part, createErr := writer.CreateFormFile("files", name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := io.WriteString(part, content); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/upload?upload_token="+url.QueryEscape(s.token), &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?status=uploaded" {
		t.Fatalf("upload: status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	for name, want := range map[string]string{"资料.txt": "existing", "资料 (1).txt": "new", "照片.jpg": "image"} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil || string(content) != want {
			t.Errorf("uploaded file %q: content=%q err=%v", name, content, readErr)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".fangxu-upload-") {
			t.Errorf("temporary upload was not removed: %q", entry.Name())
		}
	}
}

func TestUploadRequiresPageTokenAndRejectsHiddenNames(t *testing.T) {
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("not multipart"))
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("upload without page token: status=%d, want 403", response.Code)
	}
	for _, name := range []string{".secret", "../.secret", `..\\.secret`} {
		if sanitized, ok := safeUploadName(name); ok {
			t.Errorf("safeUploadName(%q) = %q, true; want rejected", name, sanitized)
		}
	}
	if sanitized, ok := safeUploadName(`..\\folder\\报告.pdf`); !ok || sanitized != "报告.pdf" {
		t.Errorf("safeUploadName traversal cleanup = %q, %v", sanitized, ok)
	}
}

func TestRemoteClientCannotChangeDirectory(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	s, err := newServer(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"admin_token": {s.token}, "action": {"apply"}, "path": {newRoot}}
	request := httptest.NewRequest(http.MethodPost, "/directory", strings.NewReader(form.Encode()))
	request.RemoteAddr = "192.168.1.20:43210"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", response.Code)
	}
	root, _ := s.roots()
	wantRoot, _ := filepath.Abs(oldRoot)
	if root != wantRoot {
		t.Fatalf("remote request changed shared root to %q", root)
	}
}

func TestInvalidDirectoryDoesNotReplaceCurrentDirectory(t *testing.T) {
	oldRoot := t.TempDir()
	s, err := newServer(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "不存在")
	form := url.Values{"admin_token": {s.token}, "action": {"apply"}, "path": {missing}}
	request := httptest.NewRequest(http.MethodPost, "/directory", strings.NewReader(form.Encode()))
	request.RemoteAddr = "127.0.0.1:43210"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?status=invalid" {
		t.Fatalf("invalid directory: status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	root, _ := s.roots()
	wantRoot, _ := filepath.Abs(oldRoot)
	if root != wantRoot {
		t.Fatalf("invalid request changed shared root to %q", root)
	}
}

func TestPageIncludesLogoAndWorkingFilenameFilter(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "report.pdf"), "pdf")
	mustWrite(t, filepath.Join(root, "notes.txt"), "text")
	mustWrite(t, filepath.Join(root, "README.md"), "markdown")
	mustWrite(t, filepath.Join(root, "photo.jpg"), "image")
	mustWrite(t, filepath.Join(root, "movie.mp4"), "video")
	mustWrite(t, filepath.Join(root, "track.flac"), "audio")
	mustWrite(t, filepath.Join(root, "book.epub"), "ebook")
	mustWrite(t, filepath.Join(root, "backup.zip"), "archive")
	mustWrite(t, filepath.Join(root, "installer.exe"), "exe")
	mustWrite(t, filepath.Join(root, "unknown.bin"), "other")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, expected := range []string{`href="/assets/logo.png"`, `src="/assets/logo.png"`, ".brand-logo{display:block;width:72px;height:72px", ".brand-logo{width:62px;height:62px}", ".file.is-hidden{display:none}", "classList.toggle('is-hidden',!show)", ".icon{display:grid;flex:none;width:42px;height:42px;place-items:center;background:transparent", ".filter.active{border-color:#c9cee6;background:#f5f6fb", `id="category-filters"`, `id="subcategory-filters"`, `data-filters="document document:pdf ebook ebook:pdf"`, `data-filters="installer installer:windows"`, `data-icon="pdf"`, `M14.5 16.3v-4.5h2`, `data-icon="text"`, `data-icon="markdown"`, `data-icon="image"`, `data-icon="video"`, `data-icon="audio"`, `data-icon="ebook"`, `data-icon="archive"`, `data-icon="windows"`, `data-icon="other"`, "renderSubcategories", "matchesSearch&&matchesCategory"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Errorf("page does not contain %q", expected)
		}
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/logo.png", nil))
	if asset.Code != http.StatusOK || asset.Header().Get("Content-Type") != "image/png" || len(asset.Body.Bytes()) == 0 {
		t.Fatalf("logo response: status=%d content-type=%q bytes=%d", asset.Code, asset.Header().Get("Content-Type"), asset.Body.Len())
	}
}

func TestFileCategoryCoversCommonFormatsAndOther(t *testing.T) {
	tests := map[string]string{
		"方案.PDF":    "document",
		"照片.HEIC":   "image",
		"演示.mp4":    "video",
		"录音.flac":   "audio",
		"读物.azw3":   "ebook",
		"备份.tar.gz": "archive",
		"安装程序.exe":  "installer",
		"README":    "other",
	}
	for name, want := range tests {
		if got := fileCategory(name); got != want {
			t.Errorf("fileCategory(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFileClassificationSupportsNestedAndOverlappingFilters(t *testing.T) {
	tests := []struct {
		name     string
		category string
		icon     string
		tags     []string
	}{
		{"手册.pdf", "document", "pdf", []string{"document:pdf", "ebook", "ebook:pdf"}},
		{"小说.docx", "document", "word", []string{"document:word", "ebook", "ebook:word"}},
		{"表格.xlsx", "document", "spreadsheet", []string{"document:spreadsheet"}},
		{"演示.pptx", "document", "presentation", []string{"document:presentation"}},
		{"说明.txt", "document", "text", []string{"document:text", "ebook", "ebook:text"}},
		{"说明.md", "document", "markdown", []string{"document:text", "ebook", "ebook:text"}},
		{"说明.html", "document", "code", []string{"document:code", "ebook", "ebook:web"}},
		{"读物.kepub.epub", "ebook", "ebook", []string{"ebook:epub"}},
		{"读物.fb2.zip", "ebook", "ebook", []string{"ebook:fb2"}},
		{"漫画.cbz", "ebook", "ebook", []string{"ebook:comic"}},
		{"备份.tar.xz", "archive", "archive", []string{"archive:tar"}},
		{"软件.msixbundle", "installer", "windows", []string{"installer:windows"}},
		{"软件.dmg", "installer", "macos", []string{"installer:macos"}},
		{"软件.AppImage", "installer", "linux", []string{"installer:linux"}},
		{"软件.apk", "installer", "android", []string{"installer:android"}},
		{"软件.ipa", "installer", "ios", []string{"installer:ios"}},
		{"字形.woff2", "other", "font", []string{"other:font"}},
		{"资料.db", "other", "database", []string{"other:database"}},
	}
	for _, test := range tests {
		classification := classifyFile(test.name)
		if classification.Category != test.category {
			t.Errorf("classifyFile(%q).Category = %q, want %q", test.name, classification.Category, test.category)
		}
		if classification.Icon != test.icon {
			t.Errorf("classifyFile(%q).Icon = %q, want %q", test.name, classification.Icon, test.icon)
		}
		for _, wantTag := range test.tags {
			if !containsString(classification.Tags, wantTag) {
				t.Errorf("classifyFile(%q).Tags = %v, missing %q", test.name, classification.Tags, wantTag)
			}
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestWeChatPageExplainsHowToOpenInBrowser(t *testing.T) {
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)

	wechatRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	wechatRequest.Header.Set("User-Agent", "Mozilla/5.0 MicroMessenger/8.0.61")
	wechatResponse := httptest.NewRecorder()
	handler.ServeHTTP(wechatResponse, wechatRequest)
	for _, expected := range []string{"微信内无法下载文件", "右上角“···”", "在浏览器打开", "Safari", "系统浏览器"} {
		if !strings.Contains(wechatResponse.Body.String(), expected) {
			t.Errorf("WeChat page does not contain %q", expected)
		}
	}

	normalResponse := httptest.NewRecorder()
	handler.ServeHTTP(normalResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(normalResponse.Body.String(), "微信内无法下载文件") {
		t.Error("normal browser page unexpectedly contains the WeChat warning")
	}
}

func TestLocalSecuritySwitchEnablesTokenProtection(t *testing.T) {
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)

	localPage := httptest.NewRequest(http.MethodGet, "/", nil)
	localPage.RemoteAddr = "127.0.0.1:43210"
	localResponse := httptest.NewRecorder()
	handler.ServeHTTP(localResponse, localPage)
	for _, expected := range []string{
		`class="token-control"`,
		"访问令牌保护",
		`role="tooltip"`,
		"开启后，访问地址和二维码会加入专属令牌",
		`role="switch" aria-checked="false"`,
		`aria-label="开启访问令牌保护"`,
		`name="enabled" value="true"`,
	} {
		if !strings.Contains(localResponse.Body.String(), expected) {
			t.Errorf("local page does not contain %q", expected)
		}
	}
	if strings.Contains(localResponse.Body.String(), `class="security"`) {
		t.Error("token protection still occupies a standalone security row")
	}

	form := "admin_token=" + s.token + "&enabled=true"
	request := httptest.NewRequest(http.MethodPost, "/security", strings.NewReader(form))
	request.RemoteAddr = "127.0.0.1:43210"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !s.isProtected() {
		t.Fatalf("enable protection: status=%d protected=%v", response.Code, s.isProtected())
	}

	protectedLocalPage := httptest.NewRequest(http.MethodGet, "/", nil)
	protectedLocalPage.RemoteAddr = "127.0.0.1:43210"
	protectedLocalResponse := httptest.NewRecorder()
	handler.ServeHTTP(protectedLocalResponse, protectedLocalPage)
	for _, expected := range []string{
		`role="switch" aria-checked="true"`,
		`aria-label="关闭访问令牌保护"`,
		`name="enabled" value="false"`,
	} {
		if !strings.Contains(protectedLocalResponse.Body.String(), expected) {
			t.Errorf("protected local page does not contain %q", expected)
		}
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("protected page without token: status %d", unauthorized.Code)
	}
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, httptest.NewRequest(http.MethodGet, "/?token="+s.token, nil))
	if authorized.Code != http.StatusOK {
		t.Fatalf("protected page with token: status %d", authorized.Code)
	}
	if strings.Contains(authorized.Body.String(), "手机访问地址 · 扫码即可打开") {
		t.Fatal("mobile protected page exposes the PC connection panel")
	}
}

func TestDownloadAndRangeRequest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "movie.bin"), "0123456789")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/download?path=movie.bin", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent {
		t.Fatalf("status %d, want 206", response.Code)
	}
	if response.Body.String() != "2345" {
		t.Fatalf("body %q", response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Content-Disposition"), "movie.bin") {
		t.Fatal("missing download filename")
	}
}

func TestChineseDownloadNameIsPreserved(t *testing.T) {
	root := t.TempDir()
	const name = "方序资料 2026（最终版）.pdf"
	mustWrite(t, filepath.Join(root, name), "pdf")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(page.Body.String(), `download="方序资料 2026（最终版）.pdf"`) {
		t.Fatal("download link does not preserve the Chinese filename")
	}

	request := httptest.NewRequest(http.MethodGet, "/download?path="+url.QueryEscape(name), nil)
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	disposition := response.Header().Get("Content-Disposition")
	for _, expected := range []string{`filename="` + name + `"`, "filename*=UTF-8''" + url.PathEscape(name)} {
		if !strings.Contains(disposition, expected) {
			t.Errorf("Content-Disposition %q does not contain %q", disposition, expected)
		}
	}
}

func TestPathTraversalIsRejected(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "shared")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(parent, "secret.txt"), "secret")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../secret.txt", "/etc/passwd", ""} {
		request := httptest.NewRequest(http.MethodGet, "/download?token="+s.token+"&path="+path, nil)
		response := httptest.NewRecorder()
		s.routes(8080).ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			body, _ := io.ReadAll(response.Result().Body)
			t.Errorf("path %q: status %d, body %q", path, response.Code, body)
		}
	}
}

func TestSymlinkOutsideRootIsNotListedOrDownloaded(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "shared")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(parent, "secret.txt")
	mustWrite(t, secret, "secret")
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := scanFiles(s.root, s.resolvedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("outside symlink was listed: %#v", files)
	}
	response := httptest.NewRecorder()
	s.routes(8080).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/download?token="+s.token+"&path=link.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("outside symlink status %d", response.Code)
	}
}

func TestScanFilesHidesDotFilesAndSortsNewestFirst(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old.txt")
	newPath := filepath.Join(root, "folder", "new.txt")
	mustWrite(t, oldPath, "old")
	mustWrite(t, newPath, "new")
	mustWrite(t, filepath.Join(root, ".hidden.txt"), "hidden")
	mustWrite(t, filepath.Join(root, ".hidden-dir", "secret.txt"), "secret")
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := scanFiles(root, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v, want only two visible files", files)
	}
	if files[0].Path != "folder/new.txt" || files[1].Path != "old.txt" {
		t.Fatalf("unexpected time order: %#v", files)
	}
}

func TestHealthAndStopAreLocalOnly(t *testing.T) {
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.instanceToken = "instance-test"
	stopped := make(chan struct{}, 1)
	s.setShutdown(func() {
		stopped <- struct{}{}
	})
	handler := s.routes(8080)

	health := httptest.NewRequest(http.MethodHead, "/health", nil)
	health.RemoteAddr = "127.0.0.1:43210"
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, health)
	if healthResponse.Code != http.StatusOK || healthResponse.Header().Get(instanceHeader) != "instance-test" {
		t.Fatalf("health response: status=%d header=%q", healthResponse.Code, healthResponse.Header().Get(instanceHeader))
	}

	remoteStop := httptest.NewRequest(http.MethodPost, "/stop", strings.NewReader("stop_token="+s.stopToken))
	remoteStop.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	remoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(remoteResponse, remoteStop)
	if remoteResponse.Code != http.StatusForbidden {
		t.Fatalf("remote stop request status=%d, want 403", remoteResponse.Code)
	}

	localStop := httptest.NewRequest(http.MethodPost, "/stop", strings.NewReader("stop_token="+s.stopToken))
	localStop.RemoteAddr = "127.0.0.1:43210"
	localStop.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	localResponse := httptest.NewRecorder()
	handler.ServeHTTP(localResponse, localStop)
	if localResponse.Code != http.StatusOK {
		t.Fatalf("local stop response: status=%d body=%q", localResponse.Code, localResponse.Body.String())
	}
	for _, expected := range []string{"文件传输服务已完全关闭", "可以放心关闭此页面", "其他设备无法再通过原地址访问", "data:image/png;base64,", "place-items:center", `class="brand"`, `class="product">方序传文件`, "width:104px;height:104px", "white-space:nowrap"} {
		if !strings.Contains(localResponse.Body.String(), expected) {
			t.Errorf("stop page does not contain %q", expected)
		}
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown was not called")
	}
}

func TestAcquireInstanceReusesRunningServiceAndReplacesStaleState(t *testing.T) {
	previousProbe := serviceProbe
	defer func() { serviceProbe = previousProbe }()
	path := filepath.Join(t.TempDir(), "service.json")
	existing := serviceState{PID: 99, Token: "existing", ControlURL: "http://127.0.0.1:3456/"}
	raw, err := json.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	serviceProbe = func(state serviceState) bool { return state.Token == existing.Token }
	claim, address, err := acquireInstance(path, 0)
	if err != nil || claim != nil || address != existing.ControlURL {
		t.Fatalf("reuse: claim=%v address=%q err=%v", claim, address, err)
	}

	serviceProbe = func(serviceState) bool { return false }
	claim, address, err = acquireInstance(path, 0)
	if err != nil || claim == nil || address != "" {
		t.Fatalf("replace stale: claim=%v address=%q err=%v", claim, address, err)
	}
	claim.release()
}

func TestHumanSize(t *testing.T) {
	tests := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1572864: "1.5 MB"}
	for input, want := range tests {
		if got := humanSize(input); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestAddressesHaveOfflinePNGQRCodes(t *testing.T) {
	const address = "http://192.168.1.8:8080/?token=test-token"
	dataURI, err := qrCodeDataURI(address)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/png;base64,"
	encoded := strings.TrimPrefix(string(dataURI), prefix)
	if encoded == string(dataURI) {
		t.Fatal("QR code is not an embedded PNG")
	}
	png, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatal("QR code does not have a PNG signature")
	}

	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	err = s.page.Execute(&rendered, pageData{Admin: true, Addresses: []accessAddress{{URL: address, QRCode: dataURI}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{address, prefix, "手机访问地址 · 扫码即可打开"} {
		if !strings.Contains(rendered.String(), expected) {
			t.Errorf("rendered page does not contain %q", expected)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
