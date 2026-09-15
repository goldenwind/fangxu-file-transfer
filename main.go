// fangxu-file-transfer shares a directory from a Windows, macOS, or Linux PC
// with phones and tablets on the same local network.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	defaultListen  = "0.0.0.0:0"
	instanceHeader = "X-Fangxu-File-Transfer-Instance"
	maxUploadBytes = 10 << 30
	maxUploadFiles = 100
)

//go:embed assets/fangxu-file-transfer-logo.png
var logoPNG []byte

type sharedFile struct {
	Name         string
	Path         string
	Folder       string
	Category     string
	Icon         string
	FilterTags   string
	Size         string
	Bytes        int64
	Modified     string
	ModifiedUnix int64
}

type pageData struct {
	Files       []sharedFile
	FileCount   int
	Root        string
	AccessToken string
	UploadToken string
	AdminToken  string
	StopToken   string
	Admin       bool
	Protected   bool
	Addresses   []accessAddress
	Status      string
	WeChat      bool
}

type accessAddress struct {
	URL    string
	QRCode template.URL
}

type server struct {
	mu            sync.RWMutex
	pickerMu      sync.Mutex
	uploadMu      sync.Mutex
	root          string
	resolvedRoot  string
	token         string
	protected     bool
	stopToken     string
	instanceToken string
	shutdown      func()
	page          *template.Template
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	directory := flag.String("dir", "", "要共享的目录（默认：Downloads）")
	listen := flag.String("listen", defaultListen, "监听地址")
	noOpen := flag.Bool("no-open", false, "启动后不打开电脑浏览器")
	flag.Parse()

	statePath, err := serviceStatePath()
	if err != nil {
		return fmt.Errorf("创建服务状态目录失败: %w", err)
	}
	claim, existingURL, err := acquireInstance(statePath, 3*time.Second)
	if err != nil {
		return fmt.Errorf("检查已运行服务失败: %w", err)
	}
	if existingURL != "" {
		fmt.Println("方序传文件服务已在运行：", existingURL)
		if !*noOpen {
			if err := openBrowser(existingURL); err != nil {
				return fmt.Errorf("无法打开已运行服务，请手动访问 %s: %w", existingURL, err)
			}
		}
		return nil
	}
	defer claim.release()

	root := *directory
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root = filepath.Join(home, "Downloads")
	}

	handler, err := newServer(root)
	if err != nil {
		return err
	}
	handler.instanceToken = claim.token
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	localURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if err := claim.publish(localURL); err != nil {
		return fmt.Errorf("保存服务地址失败: %w", err)
	}
	httpServer := &http.Server{Handler: handler.routes(port), ReadHeaderTimeout: 10 * time.Second}
	handler.setShutdown(func() { _ = httpServer.Close() })

	sharedRoot, _ := handler.roots()
	fmt.Printf("\n方序传文件\n共享目录：%s\n\n", sharedRoot)
	for _, address := range accessURLs(port, false, handler.token) {
		fmt.Println("手机访问：", address)
	}
	fmt.Println("\n服务正在运行。可在电脑页面停止传输服务，或按 Ctrl+C 退出。")

	if !*noOpen {
		go func() {
			time.Sleep(250 * time.Millisecond)
			if err := openBrowser(localURL); err != nil {
				log.Printf("打开浏览器失败: %v", err)
			}
		}()
	}
	err = httpServer.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newServer(root string) (*server, error) {
	abs, resolved, err := resolveDirectory(root)
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	stopToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	page, err := template.New("index").Parse(indexHTML)
	if err != nil {
		return nil, err
	}
	return &server{root: abs, resolvedRoot: resolved, token: token, stopToken: stopToken, page: page}, nil
}

func (s *server) routes(port int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index(port))
	mux.HandleFunc("/assets/logo.png", logo)
	mux.HandleFunc("/download", s.download)
	mux.HandleFunc("/upload", s.upload)
	mux.HandleFunc("/directory", s.changeDirectory)
	mux.HandleFunc("/security", s.changeSecurity)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/stop", s.stop)
	return securityHeaders(mux)
}

func logo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	http.ServeContent(w, r, "fangxu-file-transfer-logo.png", time.Time{}, bytes.NewReader(logoPNG))
}

func (s *server) setShutdown(shutdown func()) {
	s.mu.Lock()
	s.shutdown = shutdown
	s.mu.Unlock()
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !isLoopbackRequest(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set(instanceHeader, s.instanceToken)
	w.WriteHeader(http.StatusOK)
}

func (s *server) stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isLoopbackRequest(r) || subtle.ConstantTimeCompare([]byte(r.FormValue("stop_token")), []byte(s.stopToken)) != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	logoData := base64.StdEncoding.EncodeToString(logoPNG)
	_, _ = fmt.Fprintf(w, stoppedHTML, logoData)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	s.mu.RLock()
	shutdown := s.shutdown
	s.mu.RUnlock()
	if shutdown != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			shutdown()
		}()
	}
}

func (s *server) authorized(r *http.Request) bool {
	if isLoopbackRequest(r) || !s.isProtected() {
		return true
	}
	provided := r.URL.Query().Get("token")
	return len(provided) == len(s.token) && subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}

func (s *server) isProtected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.protected
}

func (s *server) setProtected(enabled bool) {
	s.mu.Lock()
	s.protected = enabled
	s.mu.Unlock()
}

func resolveDirectory(root string) (string, string, error) {
	if strings.TrimSpace(root) == "" {
		return "", "", errors.New("共享目录不能为空")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("共享路径不是目录: %s", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", err
	}
	return abs, resolved, nil
}

func (s *server) setRoot(root string) error {
	abs, resolved, err := resolveDirectory(root)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.root = abs
	s.resolvedRoot = resolved
	s.mu.Unlock()
	return nil
}

func (s *server) roots() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root, s.resolvedRoot
}

func (s *server) index(port int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !s.authorized(r) {
			http.Error(w, "Invalid or missing sharing link", http.StatusUnauthorized)
			return
		}
		admin := isLoopbackRequest(r)
		protected := s.isProtected()
		root, resolvedRoot := s.roots()
		files, err := scanFiles(root, resolvedRoot)
		if err != nil {
			http.Error(w, "Unable to scan the shared directory", http.StatusInternalServerError)
			return
		}
		var addresses []accessAddress
		if admin {
			addresses, err = addressesWithQRCodes(port, protected, s.token)
			if err != nil {
				http.Error(w, "Unable to generate the connection QR code", http.StatusInternalServerError)
				return
			}
		}
		data := pageData{
			Files: files, FileCount: len(files), Root: root, Admin: admin, Protected: protected,
			Addresses: addresses, Status: directoryStatus(r.URL.Query().Get("status")),
			WeChat: strings.Contains(strings.ToLower(r.UserAgent()), "micromessenger"), UploadToken: s.token,
		}
		if protected {
			data.AccessToken = s.token
		}
		if admin {
			data.AdminToken = s.token
			data.StopToken = s.stopToken
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.page.Execute(w, data); err != nil {
			log.Printf("render page: %v", err)
		}
	}
}

func (s *server) changeDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "Only the PC can change the shared directory", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil || subtle.ConstantTimeCompare([]byte(r.Form.Get("admin_token")), []byte(s.token)) != 1 {
		http.Error(w, "Invalid request", http.StatusForbidden)
		return
	}

	directory := r.Form.Get("path")
	switch r.Form.Get("action") {
	case "choose":
		s.pickerMu.Lock()
		chosen, err := chooseDirectory()
		s.pickerMu.Unlock()
		if err != nil || chosen == "" {
			http.Redirect(w, r, "/?status=cancelled", http.StatusSeeOther)
			return
		}
		directory = chosen
	case "apply":
	default:
		http.Error(w, "Invalid directory action", http.StatusBadRequest)
		return
	}
	if err := s.setRoot(directory); err != nil {
		http.Redirect(w, r, "/?status=invalid", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?status=changed", http.StatusSeeOther)
}

func directoryStatus(status string) string {
	switch status {
	case "changed":
		return "共享目录已更新。"
	case "invalid":
		return "目录无效或无法访问，请检查路径。"
	case "cancelled":
		return "未选择新目录。"
	case "uploaded":
		return "文件已上传到共享目录。"
	case "upload-empty":
		return "请先选择需要上传的文件。"
	case "upload-too-large":
		return "上传内容超过 10 GB，请减少文件数量或大小后重试。"
	case "upload-too-many":
		return "一次最多上传 100 个文件，请分批上传。"
	case "upload-invalid":
		return "部分文件名无效，请检查后重试。"
	case "upload-failed":
		return "文件上传失败，请确认共享目录仍可写入。"
	default:
		return ""
	}
}

func chooseDirectory() (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "选择共享目录")`)
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; $dialog = New-Object System.Windows.Forms.FolderBrowserDialog; $dialog.Description = '选择共享目录'; if ($dialog.ShowDialog() -eq 'OK') { [Console]::OutputEncoding = [Text.Encoding]::UTF8; Write-Output $dialog.SelectedPath }`
		command = exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			command = exec.Command(path, "--file-selection", "--directory", "--title=选择共享目录")
		} else if path, err := exec.LookPath("kdialog"); err == nil {
			command = exec.Command(path, "--getexistingdirectory", ".", "--title", "选择共享目录")
		} else {
			return "", errors.New("未找到可用的文件夹选择器，请直接输入路径")
		}
	}
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *server) changeSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "Only the PC can change access protection", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil || subtle.ConstantTimeCompare([]byte(r.Form.Get("admin_token")), []byte(s.token)) != 1 {
		http.Error(w, "Invalid request", http.StatusForbidden)
		return
	}
	enabled, err := strconv.ParseBool(r.Form.Get("enabled"))
	if err != nil {
		http.Error(w, "Invalid protection setting", http.StatusBadRequest)
		return
	}
	s.setProtected(enabled)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (s *server) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "Invalid or missing sharing link", http.StatusUnauthorized)
		return
	}
	requested := r.URL.Query().Get("path")
	root, resolvedRoot := s.roots()
	path, err := safeFilePath(root, resolvedRoot, requested)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(info.Name()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

type pendingUpload struct {
	temporaryPath string
	name          string
}

func (s *server) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("upload_token")), []byte(s.token)) != 1 {
		http.Error(w, "Invalid upload request", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		s.redirectAfterUpload(w, r, "upload-invalid")
		return
	}
	root, _ := s.roots()
	pending := make([]pendingUpload, 0, 4)
	defer func() {
		for _, upload := range pending {
			_ = os.Remove(upload.temporaryPath)
		}
	}()

	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			s.redirectAfterUpload(w, r, uploadErrorStatus(nextErr))
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		if len(pending) >= maxUploadFiles {
			_ = part.Close()
			s.redirectAfterUpload(w, r, "upload-too-many")
			return
		}
		name, valid := safeUploadName(part.FileName())
		if !valid {
			_ = part.Close()
			s.redirectAfterUpload(w, r, "upload-invalid")
			return
		}
		temporary, createErr := os.CreateTemp(root, ".fangxu-upload-*")
		if createErr != nil {
			_ = part.Close()
			s.redirectAfterUpload(w, r, "upload-failed")
			return
		}
		temporaryPath := temporary.Name()
		_, copyErr := io.Copy(temporary, part)
		closeErr := temporary.Close()
		_ = part.Close()
		pending = append(pending, pendingUpload{temporaryPath: temporaryPath, name: name})
		if copyErr != nil {
			s.redirectAfterUpload(w, r, uploadErrorStatus(copyErr))
			return
		}
		if closeErr != nil {
			s.redirectAfterUpload(w, r, "upload-failed")
			return
		}
	}
	if len(pending) == 0 {
		s.redirectAfterUpload(w, r, "upload-empty")
		return
	}

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	committed := make([]string, 0, len(pending))
	for _, upload := range pending {
		destination, pathErr := availableUploadPath(root, upload.name)
		if pathErr == nil {
			pathErr = os.Rename(upload.temporaryPath, destination)
		}
		if pathErr == nil {
			pathErr = os.Chmod(destination, 0o644)
		}
		if pathErr != nil {
			if destination != "" {
				_ = os.Remove(destination)
			}
			for _, path := range committed {
				_ = os.Remove(path)
			}
			s.redirectAfterUpload(w, r, "upload-failed")
			return
		}
		committed = append(committed, destination)
	}
	s.redirectAfterUpload(w, r, "uploaded")
}

func safeUploadName(name string) (string, bool) {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if runtime.GOOS == "windows" {
		name = strings.Map(func(character rune) rune {
			if strings.ContainsRune(`:*?"<>|`, character) {
				return '_'
			}
			return character
		}, name)
		name = strings.TrimRight(name, ". ")
	}
	return name, name != "" && name != "." && name != ".." && !strings.HasPrefix(name, ".")
}

func availableUploadPath(root, name string) (string, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for sequence := 0; sequence < 10000; sequence++ {
		candidateName := name
		if sequence > 0 {
			candidateName = fmt.Sprintf("%s (%d)%s", stem, sequence, extension)
		}
		candidate := filepath.Join(root, candidateName)
		_, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("too many files with the same name")
}

func uploadErrorStatus(err error) string {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return "upload-too-large"
	}
	return "upload-failed"
}

func (s *server) redirectAfterUpload(w http.ResponseWriter, r *http.Request, status string) {
	query := url.Values{"status": {status}}
	if s.isProtected() {
		query.Set("token", s.token)
	}
	http.Redirect(w, r, "/?"+query.Encode(), http.StatusSeeOther)
}

func scanFiles(root, resolvedRoot string) ([]sharedFile, error) {
	var files []sharedFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil || !withinRoot(resolvedRoot, resolved) {
				return nil
			}
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		folder := filepath.Dir(relative)
		if folder == "." {
			folder = ""
		}
		classification := classifyFile(entry.Name())
		files = append(files, sharedFile{
			Name: entry.Name(), Path: filepath.ToSlash(relative), Folder: filepath.ToSlash(folder),
			Category: classification.Category, Icon: classification.Icon, FilterTags: strings.Join(classification.Tags, " "),
			Size: humanSize(info.Size()), Bytes: info.Size(), Modified: info.ModTime().Format("2006-01-02 15:04"),
			ModifiedUnix: info.ModTime().UnixMilli(),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModifiedUnix != files[j].ModifiedUnix {
			return files[i].ModifiedUnix > files[j].ModifiedUnix
		}
		return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
	})
	return files, err
}

func fileCategory(name string) string {
	return classifyFile(name).Category
}

type fileClassification struct {
	Category string
	Icon     string
	Tags     []string
}

func classifyFile(name string) fileClassification {
	lowerName := strings.ToLower(name)
	extension := strings.TrimPrefix(filepath.Ext(lowerName), ".")
	category, subcategory := "other", "unclassified"
	ebookSubcategory := ""

	switch {
	case strings.HasSuffix(lowerName, ".fb2.zip"):
		category, subcategory = "ebook", "fb2"
	case strings.HasSuffix(lowerName, ".kepub.epub"):
		category, subcategory = "ebook", "epub"
	case hasAnySuffix(lowerName, ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"):
		category, subcategory = "archive", "tar"
	default:
		switch extension {
		case "pdf":
			category, subcategory, ebookSubcategory = "document", "pdf", "pdf"
		case "doc", "docx", "odt", "pages", "wps", "wpd":
			category, subcategory, ebookSubcategory = "document", "word", "word"
		case "xls", "xlsx", "xlsm", "ods", "csv", "numbers":
			category, subcategory = "document", "spreadsheet"
		case "ppt", "pptx", "odp", "key":
			category, subcategory = "document", "presentation"
		case "txt", "md", "markdown", "rtf":
			category, subcategory, ebookSubcategory = "document", "text", "text"
		case "html", "htm", "xhtml", "mhtml":
			category, subcategory, ebookSubcategory = "document", "code", "web"
		case "json", "xml", "yaml", "yml", "toml", "ini", "conf", "log", "tex", "go", "rs", "py", "js", "jsx", "ts", "tsx", "java", "kt", "kts", "swift", "c", "h", "cc", "cpp", "hpp", "cs", "php", "rb", "sh", "zsh", "fish", "bat", "cmd", "ps1", "sql", "css", "scss", "sass", "less", "vue", "svelte":
			category, subcategory = "document", "code"
		case "jpg", "jpeg":
			category, subcategory = "image", "jpeg"
		case "png":
			category, subcategory = "image", "png"
		case "gif":
			category, subcategory = "image", "gif"
		case "webp":
			category, subcategory = "image", "webp"
		case "heic", "heif":
			category, subcategory = "image", "heic"
		case "svg":
			category, subcategory = "image", "svg"
		case "bmp", "tif", "tiff", "avif", "ico", "raw", "dng":
			category, subcategory = "image", "other"
		case "mp4", "m4v":
			category, subcategory = "video", "mp4"
		case "mov":
			category, subcategory = "video", "mov"
		case "mkv":
			category, subcategory = "video", "mkv"
		case "webm":
			category, subcategory = "video", "webm"
		case "avi":
			category, subcategory = "video", "avi"
		case "flv", "wmv", "mpeg", "mpg", "3gp", "mts", "m2ts":
			category, subcategory = "video", "other"
		case "mp3":
			category, subcategory = "audio", "mp3"
		case "m4a", "aac":
			category, subcategory = "audio", "aac"
		case "flac":
			category, subcategory = "audio", "flac"
		case "wav":
			category, subcategory = "audio", "wav"
		case "ogg", "opus":
			category, subcategory = "audio", "ogg"
		case "wma", "aiff", "ape", "amr":
			category, subcategory = "audio", "other"
		case "epub":
			category, subcategory = "ebook", "epub"
		case "mobi", "azw", "azw3", "kfx", "prc":
			category, subcategory = "ebook", "kindle"
		case "fb2":
			category, subcategory = "ebook", "fb2"
		case "djvu", "djv":
			category, subcategory = "ebook", "djvu"
		case "cbz", "cbr", "cb7", "cbt":
			category, subcategory = "ebook", "comic"
		case "chm":
			category, subcategory = "ebook", "web"
		case "pdb", "lit", "lrf", "tcr":
			category, subcategory = "ebook", "other"
		case "zip":
			category, subcategory = "archive", "zip"
		case "rar":
			category, subcategory = "archive", "rar"
		case "7z":
			category, subcategory = "archive", "7z"
		case "tar", "tgz", "tbz", "tbz2", "txz":
			category, subcategory = "archive", "tar"
		case "gz", "bz2", "xz", "zst", "lz", "lz4":
			category, subcategory = "archive", "compressed"
		case "exe", "msi", "msix", "appx", "appxbundle", "msixbundle", "msu":
			category, subcategory = "installer", "windows"
		case "dmg", "pkg", "mpkg":
			category, subcategory = "installer", "macos"
		case "deb", "rpm", "appimage", "snap", "flatpak", "flatpakref":
			category, subcategory = "installer", "linux"
		case "apk", "xapk", "apks", "aab":
			category, subcategory = "installer", "android"
		case "ipa":
			category, subcategory = "installer", "ios"
		case "iso", "img":
			category, subcategory = "other", "disk-image"
		case "ttf", "otf", "woff", "woff2", "eot":
			category, subcategory = "other", "font"
		case "psd", "ai", "sketch", "fig", "xd":
			category, subcategory = "other", "design"
		case "db", "sqlite", "sqlite3":
			category, subcategory = "other", "database"
		}
	}

	tags := []string{category, category + ":" + subcategory}
	if ebookSubcategory != "" {
		tags = append(tags, "ebook", "ebook:"+ebookSubcategory)
	}
	return fileClassification{Category: category, Icon: fileIcon(lowerName, extension, category, subcategory), Tags: tags}
}

func fileIcon(lowerName, extension, category, subcategory string) string {
	switch extension {
	case "md", "markdown":
		return "markdown"
	case "txt", "rtf", "log":
		return "text"
	}

	switch category {
	case "document":
		switch subcategory {
		case "pdf":
			return "pdf"
		case "word":
			return "word"
		case "spreadsheet":
			return "spreadsheet"
		case "presentation":
			return "presentation"
		case "code":
			return "code"
		default:
			return "document"
		}
	case "image", "video", "audio", "archive":
		return category
	case "ebook":
		return "ebook"
	case "installer":
		return subcategory
	case "other":
		switch subcategory {
		case "disk-image", "font", "design", "database":
			return subcategory
		}
	}
	if strings.HasSuffix(lowerName, ".fb2.zip") {
		return "ebook"
	}
	return "other"
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func safeFilePath(root, resolvedRoot, requested string) (string, error) {
	if requested == "" || filepath.IsAbs(requested) {
		return "", os.ErrNotExist
	}
	clean := filepath.Clean(filepath.FromSlash(requested))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", os.ErrNotExist
	}
	candidate := filepath.Join(root, clean)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !withinRoot(resolvedRoot, resolved) {
		return "", os.ErrNotExist
	}
	return resolved, nil
}

func withinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func randomToken() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type serviceState struct {
	PID        int    `json:"pid"`
	Token      string `json:"token"`
	ControlURL string `json:"control_url,omitempty"`
}

type instanceClaim struct {
	path  string
	token string
}

var serviceProbe = probeService

func serviceStatePath() (string, error) {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	// Keep the legacy cache location so existing running instances remain discoverable.
	directory := filepath.Join(cacheDirectory, "pc-mobile-transfer")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(directory, "service.json"), nil
}

func acquireInstance(path string, wait time.Duration) (*instanceClaim, string, error) {
	token, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	state := serviceState{PID: os.Getpid(), Token: token}
	stateData, err := json.Marshal(state)
	if err != nil {
		return nil, "", err
	}
	deadline := time.Now().Add(wait)

	for {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr == nil {
			if _, err := file.Write(stateData); err != nil {
				file.Close()
				_ = os.Remove(path)
				return nil, "", err
			}
			if err := file.Close(); err != nil {
				_ = os.Remove(path)
				return nil, "", err
			}
			return &instanceClaim{path: path, token: token}, "", nil
		}
		if !errors.Is(createErr, os.ErrExist) {
			return nil, "", createErr
		}

		existing, raw, readErr := readServiceState(path)
		if readErr == nil && serviceProbe(existing) {
			return nil, existing.ControlURL, nil
		}
		if time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		removed, err := removeStateIfUnchanged(path, raw)
		if err != nil {
			return nil, "", err
		}
		if !removed {
			deadline = time.Now().Add(wait)
		}
	}
}

func readServiceState(path string) (serviceState, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return serviceState{}, nil, err
	}
	var state serviceState
	if err := json.Unmarshal(raw, &state); err != nil {
		return serviceState{}, raw, err
	}
	if state.Token == "" {
		return serviceState{}, raw, errors.New("service state has no instance token")
	}
	return state, raw, nil
}

func removeStateIfUnchanged(path string, observed []byte) (bool, error) {
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(current, observed) {
		return false, nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func probeService(state serviceState) bool {
	parsed, err := url.Parse(state.ControlURL)
	if err != nil || parsed.Scheme != "http" || parsed.Port() == "" || !isLoopbackHost(parsed.Hostname()) {
		return false
	}
	request, err := http.NewRequest(http.MethodHead, strings.TrimRight(state.ControlURL, "/")+"/health", nil)
	if err != nil {
		return false
	}
	response, err := (&http.Client{Timeout: 250 * time.Millisecond}).Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK && subtle.ConstantTimeCompare([]byte(response.Header.Get(instanceHeader)), []byte(state.Token)) == 1
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (claim *instanceClaim) publish(controlURL string) error {
	state, _, err := readServiceState(claim.path)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(state.Token), []byte(claim.token)) != 1 {
		return errors.New("service state was claimed by another process")
	}
	state.ControlURL = controlURL
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(claim.path, data, 0o600)
}

func (claim *instanceClaim) release() {
	state, raw, err := readServiceState(claim.path)
	if err != nil || subtle.ConstantTimeCompare([]byte(state.Token), []byte(claim.token)) != 1 {
		return
	}
	_, _ = removeStateIfUnchanged(claim.path, raw)
}

// lanAddressScore excludes addresses that cannot normally serve peers on a LAN.
// Higher scores prefer private addresses on the default route, then Wi-Fi/Ethernet.
func lanAddressScore(iface net.Interface, ip net.IP, network *net.IPNet, preferred net.IP) (int, bool) {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagPointToPoint != 0 {
		return 0, false
	}
	name := strings.ToLower(iface.Name)
	for _, prefix := range []string{"utun", "tun", "tap", "ppp", "ipsec", "wg", "tailscale", "zt", "docker", "veth", "virbr", "vmnet", "vbox", "br-", "bridge", "awdl", "llw", "anpi", "p2p", "ham", "vethernet"} {
		if strings.HasPrefix(name, prefix) {
			return 0, false
		}
	}
	for _, marker := range []string{"vpn", "virtual", "vmware", "virtualbox", "hyper-v", "wsl", "loopback", "bluetooth"} {
		if strings.Contains(name, marker) {
			return 0, false
		}
	}
	v4 := ip.To4()
	if v4 == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || v4[0] == 0 || v4[0] >= 224 {
		return 0, false
	}
	// Shared carrier, documentation and benchmark ranges are commonly assigned
	// by VPNs or test networks rather than a device-accessible LAN.
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24"} {
		_, blocked, _ := net.ParseCIDR(cidr)
		if blocked.Contains(ip) {
			return 0, false
		}
	}
	if network != nil {
		ones, bits := network.Mask.Size()
		if bits == 32 && ones < 31 {
			networkIP := v4.Mask(network.Mask)
			broadcast := append(net.IP(nil), networkIP...)
			for i := range broadcast {
				broadcast[i] |= ^network.Mask[i]
			}
			if v4.Equal(networkIP) || v4.Equal(broadcast) {
				return 0, false
			}
		}
	}
	score := 0
	if ip.IsPrivate() {
		score += 400
	}
	if ip.Equal(preferred) {
		score += 200
	}
	switch {
	case strings.HasPrefix(name, "wl"), strings.HasPrefix(name, "wifi"), strings.HasPrefix(name, "wi-fi"), strings.HasPrefix(name, "wlan"):
		score += 120
	case strings.HasPrefix(name, "en"), strings.HasPrefix(name, "eth"), strings.HasPrefix(name, "ethernet"):
		score += 100
	}
	return score, true
}

// Connecting UDP selects a local source using the OS routing table without
// sending a packet; offline hosts simply fall back to interface-based ranking.
func defaultRouteIP() net.IP {
	conn, err := net.DialTimeout("udp4", "1.1.1.1:53", 200*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()
	if address, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return address.IP
	}
	return nil
}

func accessURLs(port int, protected bool, token string) []string {
	type candidate struct {
		ip    net.IP
		score int
	}
	seen := map[string]bool{}
	var candidates []candidate
	preferred := defaultRouteIP()
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, network, err := net.ParseCIDR(address.String())
			if err != nil {
				continue
			}
			score, usable := lanAddressScore(iface, ip, network, preferred)
			if !usable || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			candidates = append(candidates, candidate{ip, score})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return bytes.Compare(candidates[i].ip.To4(), candidates[j].ip.To4()) < 0
	})
	urls := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		u := fmt.Sprintf("http://%s:%d/", candidate.ip.String(), port)
		if protected {
			u += "?token=" + url.QueryEscape(token)
		}
		urls = append(urls, u)
	}
	return urls
}

func addressesWithQRCodes(port int, protected bool, token string) ([]accessAddress, error) {
	urls := accessURLs(port, protected, token)
	addresses := make([]accessAddress, 0, len(urls))
	for _, address := range urls {
		dataURI, err := qrCodeDataURI(address)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, accessAddress{URL: address, QRCode: dataURI})
	}
	return addresses, nil
}

func qrCodeDataURI(address string) (template.URL, error) {
	png, err := qrcode.Encode(address, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png)), nil
}

func contentDisposition(name string) string {
	escaped := url.PathEscape(name)
	quotedUTF8 := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if quotedUTF8 == "" {
		quotedUTF8 = "download"
	}
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", quotedUTF8, escaped)
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit || suffix == "TB" {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return ""
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Start()
}

const stoppedHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>服务已停止 · 方序传文件</title><style>
*{box-sizing:border-box}body{display:grid;min-height:100vh;margin:0;padding:24px;place-items:center;background:#f3f4f7;color:#172039;font:15px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",Arial,sans-serif}.card{width:min(100%%,720px);padding:38px 44px;border:1px solid #dfe2e9;border-radius:8px;background:#fff;box-shadow:0 8px 28px rgba(23,32,57,.07);overflow-x:auto;text-align:center}.brand{display:flex;align-items:center;justify-content:center;gap:20px;margin-bottom:26px}.logo{display:block;width:104px;height:104px;object-fit:contain}.brand-copy{text-align:left}.product{display:block;font-size:29px;font-weight:780;letter-spacing:-.03em;white-space:nowrap}.tagline{display:block;margin-top:3px;color:#70778a;font-size:13px;font-weight:600;letter-spacing:.12em;white-space:nowrap}.content{padding-top:22px;border-top:1px solid #e7e9ee}.state{display:inline-flex;align-items:center;gap:7px;margin-bottom:12px;color:#2c7250;font-size:13px;font-weight:700;white-space:nowrap}.dot{width:7px;height:7px;border-radius:50%%;background:#319364}h1{margin:0 0 10px;font-size:25px;letter-spacing:-.02em;white-space:nowrap}p{margin:0;color:#70778a;white-space:nowrap}.hint{margin-top:18px;padding-top:16px;border-top:1px solid #e7e9ee;font-size:13px}
</style></head><body><main class="card"><div class="brand"><img class="logo" src="data:image/png;base64,%s" alt="方序传文件 Logo"><span class="brand-copy"><span class="product">方序传文件</span><span class="tagline">方寸之间，传递有序</span></span></div><div class="content"><div class="state"><span class="dot"></span>已安全停止</div><h1>文件传输服务已完全关闭</h1><p>电脑上的文件已停止共享，其他设备无法再通过原地址访问。</p><p class="hint">你可以放心关闭此页面。下次需要传文件时，再次双击“方序传文件”即可。</p></div></main></body></html>`

const indexHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="方序传文件是一款免费、简洁的局域网文件传输工具，让电脑上的文件通过同一 Wi-Fi 轻松传到手机或平板。">
<link rel="icon" type="image/png" href="/assets/logo.png">
<title>方序传文件</title>
<style>
:root{--bg:#f3f4f7;--card:#fff;--text:#172039;--muted:#70778a;--line:#dfe2e9;--line-strong:#cbd0dc;--accent:#3446b7;--accent-soft:#f3f5ff;--success:#319364;--danger:#a83a3a}
*{box-sizing:border-box}body{margin:0;padding:20px;background:var(--bg);color:var(--text);font:15px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",Arial,sans-serif}
button,input,select{font:inherit}button{cursor:pointer}main{width:100%;min-height:calc(100vh - 40px);margin:0 auto}.hero{margin-bottom:28px}.brand{display:flex;align-items:center;gap:16px;margin-bottom:22px;padding:0 0 16px;border-bottom:1px solid var(--line)}.brand-logo{display:block;width:72px;height:72px;flex:none;object-fit:contain;transform:scale(1.45);transform-origin:center}.brand-copy{min-width:0}.brand h1{margin:0;color:var(--text);font-size:27px;font-weight:760;letter-spacing:-.03em}.tagline{margin:3px 0 0;color:var(--muted);font-size:13px;font-weight:600;letter-spacing:.12em}
.admin-panel{padding:20px;border:1px solid var(--line);border-radius:6px;background:var(--card);box-shadow:0 4px 14px rgba(23,32,57,.04)}.admin-head{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}.admin-copy{min-width:0}.admin-copy h2{margin:0 0 7px;color:var(--text);font-size:22px;letter-spacing:-.01em}.admin-copy p{margin:0;color:var(--muted);line-height:1.65}.service{display:flex;flex:none;align-items:center;justify-content:flex-end;gap:11px;flex-wrap:wrap}.service-state{display:inline-flex;align-items:center;color:#2c7250;font-size:14px;font-weight:700;white-space:nowrap}.service-dot{width:7px;height:7px;margin-right:7px;border-radius:50%;background:var(--success)}.service form{margin:0}.token-control{display:flex;align-items:center;gap:6px;padding-left:11px;border-left:1px solid var(--line)}.token-label{color:#454d62;font-size:13px;font-weight:680;white-space:nowrap}.token-help{position:relative;display:inline-flex}.token-help-trigger{display:grid;width:18px;height:18px;padding:0;place-items:center;border:1px solid #c9ceda;border-radius:50%;background:#fff;color:#737b8f;font-size:12px;font-weight:750;line-height:1}.token-help-trigger:hover,.token-help-trigger:focus{border-color:var(--accent);color:var(--accent);outline:none}.token-help-tip{position:absolute;z-index:10;top:calc(100% + 9px);right:-72px;width:280px;padding:10px 12px;border:1px solid #d8dce6;border-radius:5px;background:#20283c;color:#fff;font-size:12px;font-weight:500;line-height:1.6;box-shadow:0 8px 24px rgba(23,32,57,.18);opacity:0;pointer-events:none;transform:translateY(-3px);transition:opacity .15s,transform .15s;visibility:hidden}.token-help-tip:before{position:absolute;right:76px;top:-5px;width:9px;height:9px;background:#20283c;content:"";transform:rotate(45deg)}.token-help:hover .token-help-tip,.token-help:focus-within .token-help-tip{opacity:1;transform:translateY(0);visibility:visible}.token-switch{position:relative;display:block;width:38px;height:22px;padding:0;border:1px solid #b9bfcc;border-radius:11px;background:#c7cbd4;transition:border-color .15s,background .15s}.token-switch-knob{position:absolute;top:2px;left:2px;width:16px;height:16px;border-radius:50%;background:#fff;box-shadow:0 1px 3px rgba(23,32,57,.25);transition:transform .15s}.token-switch:hover,.token-switch:focus{border-color:#7c87bd;outline:2px solid rgba(52,70,183,.12);outline-offset:2px}.token-switch.on{border-color:var(--accent);background:var(--accent)}.token-switch.on .token-switch-knob{transform:translateX(16px)}.stop{margin:0;padding:8px 12px;border:1px solid #d39b9b;border-radius:5px;background:#fff;color:var(--danger);font-size:14px;font-weight:650}.stop:hover,.stop:focus{border-color:var(--danger);background:#fff8f8;outline:none}
.directory{margin-top:16px;padding-top:14px;border-top:1px solid #e7e9ee}.directory label{display:block;margin-bottom:7px;color:var(--muted);font-size:13px;font-weight:650}.directory-note{margin-left:7px;color:#8a90a0;font-weight:500}.directory-row{display:flex;align-items:stretch;gap:8px}.directory input[type=text]{min-width:0;flex:1;padding:9px 11px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--text);font:13px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace}.directory input[type=text]:focus{border-color:var(--accent);outline:2px solid rgba(52,70,183,.1)}.directory-actions{display:flex;gap:8px}.directory button{padding:9px 12px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--accent);font-size:13px;font-weight:700;white-space:nowrap}.directory button:hover,.directory button:focus{border-color:var(--accent);outline:none}.directory .apply{border-color:var(--accent);background:var(--accent);color:#fff}.status{margin:0 0 14px;padding:9px 11px;border-left:3px solid var(--success);background:#f3faf6;color:#2c7250;font-size:13px}
.addresses{display:grid;grid-template-columns:repeat(auto-fit,minmax(290px,1fr));gap:10px;margin-top:14px}.address{display:flex;align-items:center;gap:14px;min-width:0;padding:13px;border:1px solid var(--line);border-radius:5px;background:#f7f8fb}.address img{display:block;flex:none;width:104px;height:104px;padding:4px;border:1px solid #e5e7ec;background:#fff;object-fit:contain}.address-info{min-width:0;flex:1}.address-label{display:block;margin-bottom:5px;color:var(--muted);font-size:12px}.address code{display:block;color:#29345f;font:12px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace;overflow-wrap:anywhere}.address button{margin:10px 0 0;padding:7px 11px;border:1px solid var(--line-strong);border-radius:4px;background:#fff;color:var(--accent);font-size:13px;font-weight:700}.address button:hover,.address button:focus{border-color:var(--accent);outline:none}.wechat-warning{display:block;margin:0 0 14px;padding:14px 16px;border-left:3px solid #d78b24;background:#fff9ee;color:#5e461f}.wechat-warning strong,.wechat-warning span{display:block}.wechat-warning strong{margin-bottom:3px;color:#7a5015}.wechat-warning span{font-size:13px;line-height:1.65}.mobile-intro{flex:1;min-width:0;max-width:170px;margin:0 0 0 auto;color:var(--muted);font-size:13px;line-height:1.6;text-align:right;text-wrap:balance}
.upload-panel{display:flex;align-items:center;justify-content:space-between;gap:20px;margin:0 0 14px;padding:15px 16px;border:1px solid var(--line);border-radius:6px;background:var(--card);box-shadow:0 3px 12px rgba(23,32,57,.035)}.upload-copy{display:flex;align-items:center;gap:11px;min-width:0}.upload-icon{display:grid;flex:none;width:38px;height:38px;place-items:center;color:var(--accent)}.upload-icon svg{width:30px;height:30px}.upload-text{min-width:0}.upload-copy strong,.upload-description{display:block}.upload-copy strong{font-size:15px}.upload-description{margin-top:2px;color:var(--muted);font-size:12px}.upload-form{display:flex;align-items:center;justify-content:flex-end;gap:8px;min-width:0}.upload-picker{display:inline-flex;flex:none;align-items:center;padding:8px 11px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--accent);font-size:13px;font-weight:700;cursor:pointer}.upload-picker:hover,.upload-picker:focus-within{border-color:var(--accent)}.upload-picker input{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}.upload-selection{max-width:220px;color:var(--muted);font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.upload-submit{flex:none;padding:8px 12px;border:1px solid var(--accent);border-radius:5px;background:var(--accent);color:#fff;font-size:13px;font-weight:700}.upload-submit:hover,.upload-submit:focus{background:#293a9f;outline:none}.upload-submit:disabled{cursor:wait;opacity:.65}
.toolbar{display:flex;align-items:center;gap:10px;margin:0 0 10px}.search{width:100%;padding:11px 13px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--text)}.search:focus{border-color:var(--accent);outline:2px solid rgba(52,70,183,.1)}.sort{flex:none;padding:11px 13px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--accent);font-size:14px;font-weight:700;white-space:nowrap}.sort:hover,.sort:focus{border-color:var(--accent);outline:none}.count{min-width:64px;color:var(--muted);font-size:13px;text-align:right;white-space:nowrap}.filter-stack{margin:0 0 13px}.filter-level{display:flex;align-items:center;gap:10px;min-width:0}.filter-level+.filter-level{margin-top:7px}.filter-level[hidden]{display:none}.filter-label{flex:none;width:28px;color:#858b99;font-size:12px;font-weight:650}.filters{display:flex;min-width:0;flex:1;gap:6px;padding:2px 0;overflow-x:auto;scrollbar-width:thin}.filter{display:inline-flex;flex:none;align-items:center;gap:7px;padding:7px 10px;border:1px solid transparent;border-radius:4px;background:transparent;color:#697184;font-size:13px;font-weight:620;white-space:nowrap;transition:border-color .15s,background .15s,color .15s,box-shadow .15s}.filter-count{min-width:auto;padding:0;background:transparent;color:#9a9fad;font-size:11px;font-variant-numeric:tabular-nums;line-height:18px;text-align:center}.filter:hover,.filter:focus{border-color:#d6d9e2;background:#fff;color:#344166;outline:none}.filter.active{border-color:#c9cee6;background:#f5f6fb;color:#293b9f;box-shadow:inset 0 -2px 0 var(--accent)}.filter.active .filter-count{background:transparent;color:#6976af}.subfilters .filter{background:transparent}.subfilters .filter:hover,.subfilters .filter:focus{background:#fff}.subfilters .filter.active{background:#f5f6fb}
.batch-toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:12px;margin:0 0 12px;color:var(--muted);font-size:13px}.batch-status{margin:0 0 12px;color:var(--muted);font-size:12px}.batch-status[hidden]{display:none}.select-all{display:flex;align-items:center;gap:7px;color:var(--text);cursor:pointer}.batch-toolbar input,.file-checkbox{width:18px;height:18px;accent-color:var(--accent);cursor:pointer}.batch-toolbar button{padding:7px 10px;border:1px solid var(--line-strong);border-radius:5px;background:#fff;color:var(--accent);font-size:13px}.batch-toolbar button:disabled{opacity:.45;cursor:default}#batch-download{background:var(--accent);border-color:var(--accent);color:#fff}.file-row{display:flex;align-items:center;gap:6px;min-width:0}.file-row[hidden]{display:none}.file-select{display:grid;flex:none;place-items:center;align-self:stretch;width:30px;cursor:pointer}.file-row .file{min-width:0;flex:1}.file-row:has(.file-checkbox:checked) .file{border-color:var(--accent);background:var(--accent-soft)}.list{display:grid;gap:8px}.file{display:flex;align-items:center;gap:9px;width:100%;padding:12px 14px;border:1px solid var(--line);border-radius:5px;background:var(--card);color:inherit;text-decoration:none;transition:border-color .15s,background .15s,box-shadow .15s}.file.is-hidden{display:none}.file:hover,.file:focus{border-color:#9ea7ca;background:#fafbfe;box-shadow:0 3px 10px rgba(23,32,57,.05);outline:none}.icon{display:grid;flex:none;width:42px;height:42px;place-items:center;background:transparent;color:#677083;opacity:.9;transition:color .15s,opacity .15s}.icon svg{display:block;width:39px;height:39px}.file:hover .icon,.file:focus .icon{opacity:1}.icon.document{color:#43539a}.icon.image{color:#2f765f}.icon.video{color:#9a4c59}.icon.audio{color:#7b528e}.icon.ebook{color:#8c672d}.icon.archive{color:#766044}.icon.installer{color:#356d89}.details{min-width:0;flex:1}.name{display:block;font-size:17px;font-weight:700;line-height:1.35;overflow-wrap:anywhere}.meta{display:block;margin-top:5px;color:var(--muted);font-size:12px;overflow-wrap:anywhere}.download{flex:none;margin-left:10px;padding:7px 11px;border:1px solid #cbd2eb;border-radius:4px;background:var(--accent-soft);color:var(--accent);font-size:13px;font-weight:700}.empty{padding:36px 16px;border:1px dashed #c5cad5;border-radius:5px;background:#f8f9fb;color:var(--muted);text-align:center}.filter-empty[hidden]{display:none}.note{margin:24px 0 0;padding:14px 4px 4px;border-top:1px solid var(--line);color:var(--muted);font-size:12px;line-height:1.7}
@media(max-width:600px){body{padding:10px}.hero{margin-bottom:16px}.brand{gap:10px;margin:2px 3px 16px;padding-bottom:13px}.brand-copy{flex:none}.mobile-intro{font-size:11px;max-width:110px}.brand-logo{width:62px;height:62px}.brand h1{font-size:23px}.tagline{font-size:12px}.admin-panel{padding:15px}.admin-head{display:block}.service{justify-content:flex-start;margin-top:14px;padding-top:12px;border-top:1px solid var(--line)}.token-control{padding-left:9px}.directory-row{display:block}.directory-actions{margin-top:8px}.directory-actions button{flex:1}.addresses{grid-template-columns:1fr}.address img{width:92px;height:92px}.upload-panel{display:block}.upload-form{justify-content:flex-start;margin-top:13px;padding-top:12px;border-top:1px solid var(--line)}.upload-selection{min-width:0;flex:1}.toolbar{flex-wrap:wrap}.search{flex-basis:100%}.sort{flex:1}.file{padding:10px 9px}.icon{width:38px;height:38px}.icon svg{width:35px;height:35px}.name{font-size:16px}.download{margin-left:4px;padding:7px 9px}}

.support{margin:0 0 26px;padding:20px;border:1px solid var(--line);border-radius:6px;background:var(--card)}.support-heading{margin-bottom:16px}.support-heading h2{margin:0 0 6px;font-size:22px}.support-heading p{margin:0;color:var(--muted);line-height:1.7}.support-grid{display:flex;flex-wrap:wrap;gap:12px}.support-card{display:flex;flex:1 1 240px;align-items:center;min-width:0;padding:14px;border:1px solid var(--line);border-radius:5px;background:var(--card)}.support-card img{display:block;flex:none;width:116px;height:116px;padding:4px;border:1px solid var(--line);background:#fff;object-fit:contain}.support-card-copy{min-width:0;margin-left:14px}.support-card-title{display:block;font-size:17px;line-height:1.4}.support-card-note{display:block;margin-top:6px;color:var(--muted);font-size:13px;line-height:1.55}.support-card-public{border-color:var(--line-strong);background:var(--accent-soft)}
@media(max-width:600px){.support{margin-bottom:20px;padding:15px}.support-card{flex-basis:100%;padding:11px}.support-card img{width:104px;height:104px}.support-card-copy{margin-left:12px}}
</style></head><body><main>
<section class="hero"><header class="brand"><img class="brand-logo" src="/assets/logo.png" alt=""><span class="brand-copy"><h1>方序传文件</h1><span class="tagline">方寸之间，传递有序</span></span>{{if not .Admin}}<span class="mobile-intro">轻点文件，即可保存到当前设备。</span>{{end}}</header>
{{if .WeChat}}<aside class="wechat-warning" role="alert"><strong>微信内无法下载文件</strong><span>请点击右上角“···”，选择“在浏览器打开”。iPhone 可使用 Safari，Android 可使用系统浏览器，然后再点击文件下载。</span></aside>{{end}}
{{if .Admin}}<div class="admin-panel"><div class="admin-head"><div class="admin-copy"><h2>电脑端传文件设置</h2><p><strong>连接提醒：</strong>让手机或平板与电脑连接同一 Wi-Fi，然后扫描下方二维码。</p></div><div class="service"><span class="service-state"><span class="service-dot"></span>服务运行中</span><form class="token-control" method="post" action="/security"><input type="hidden" name="admin_token" value="{{.AdminToken}}"><input type="hidden" name="enabled" value="{{if .Protected}}false{{else}}true{{end}}"><span class="token-label">访问令牌保护</span><span class="token-help"><button class="token-help-trigger" type="button" aria-label="访问令牌保护说明" aria-describedby="token-help-tip">?</button><span class="token-help-tip" id="token-help-tip" role="tooltip">开启后，访问地址和二维码会加入专属令牌，只有拿到完整链接的人才能查看和下载文件；关闭后，同一局域网内的设备可直接访问。</span></span><button class="token-switch {{if .Protected}}on{{end}}" type="submit" role="switch" aria-checked="{{if .Protected}}true{{else}}false{{end}}" aria-label="{{if .Protected}}关闭访问令牌保护{{else}}开启访问令牌保护{{end}}"><span class="token-switch-knob"></span></button></form><form method="post" action="/stop" onsubmit="return confirm('确定停止方序传文件服务吗？')"><input type="hidden" name="stop_token" value="{{.StopToken}}"><button class="stop" type="submit">停止传输服务</button></form></div></div>
<form class="directory" method="post" action="/directory"><input type="hidden" name="admin_token" value="{{.AdminToken}}"><label for="shared-directory">共享目录：<span class="directory-note">该目录下的文件可在其他设备访问和下载</span></label><div class="directory-row"><input id="shared-directory" type="text" name="path" value="{{.Root}}" autocomplete="off" spellcheck="false"><span class="directory-actions"><button type="submit" name="action" value="choose">选择文件夹…</button><button class="apply" type="submit" name="action" value="apply">应用路径</button></span></div></form>
<div class="addresses">{{range .Addresses}}<div class="address"><img src="{{.QRCode}}" alt="方序传文件访问二维码"><div class="address-info"><span class="address-label">手机访问地址 · 扫码即可打开</span><code>{{.URL}}</code><button type="button" data-copy="{{.URL}}">复制地址</button></div></div>{{else}}<p>未发现局域网 IPv4 地址，请确认电脑已连接 Wi-Fi。</p>{{end}}</div></div>{{end}}</section>
{{if .Status}}<div class="status" role="status">{{.Status}}</div>{{end}}
{{if .Admin}}<section class="support" aria-labelledby="support-title">
      <div class="support-heading">
        <h2 id="support-title">支持与关注</h2>
        <p>方序传文件是一个免费项目。如果它为你带来了便利，欢迎请作者喝杯咖啡。感谢大家的支持，让这个免费项目可以持续维护与改进。</p>
      </div>
      <div class="support-grid">
        <div class="support-card">
          <img src="https://cdn.ip21.cn/img/common/alipay-qrcode.jpg" alt="支付宝收款二维码" width="116" height="116" loading="lazy">
          <span class="support-card-copy"><strong class="support-card-title">支付宝赞赏</strong><span class="support-card-note">打开支付宝扫一扫，感谢你的支持。</span></span>
        </div>
        <div class="support-card">
          <img src="https://cdn.ip21.cn/img/common/wechatpay-qrcode.jpg" alt="微信收款二维码" width="116" height="116" loading="lazy">
          <span class="support-card-copy"><strong class="support-card-title">微信赞赏</strong><span class="support-card-note">打开微信扫一扫，感谢你的支持。</span></span>
        </div>
        <div class="support-card support-card-public">
          <img src="https://cdn.ip21.cn/img/common/wechat-pub.png" alt="一灯 AI 微信公众号二维码" width="116" height="116" loading="lazy">
          <span class="support-card-copy"><strong class="support-card-title">关注「一灯 AI」</strong><span class="support-card-note">微信扫码关注公众号，获取 AI 工具与效率实践。</span></span>
        </div>
      </div>
    </section>
{{end}}
<section class="upload-panel" aria-labelledby="upload-title"><div class="upload-copy"><span class="upload-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><path d="M12 15V4m0 0L8 8m4-4 4 4M5 13v5.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V13" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg></span><div class="upload-text"><strong id="upload-title">上传文件</strong><span class="upload-description">从当前设备选择文件，上传到电脑的共享目录</span></div></div><form class="upload-form" method="post" enctype="multipart/form-data" action="/upload?upload_token={{.UploadToken | urlquery}}{{if .Protected}}&amp;token={{.AccessToken | urlquery}}{{end}}"><label class="upload-picker"><input id="upload-files" type="file" name="files" multiple><span>选择文件</span></label><span class="upload-selection" id="upload-selection">尚未选择</span><button class="upload-submit" type="submit">开始上传</button></form></section>
<div class="toolbar"><input id="search" class="search" type="search" placeholder="搜索文件名或文件夹…" autocomplete="off"><select id="sort" class="sort" aria-label="当前排序方式"><option value="time-desc">时间：从新到旧</option><option value="time-asc">时间：从旧到新</option><option value="name-asc">名称：正序</option><option value="name-desc">名称：倒序</option></select><span class="count"><b id="visible">{{.FileCount}}</b> / {{.FileCount}}</span></div>
<div class="filter-stack"><div class="filter-level"><span class="filter-label">类型</span><div class="filters" id="category-filters" role="group" aria-label="按文件大类筛选"></div></div><div class="filter-level" id="subcategory-level" hidden><span class="filter-label">格式</span><div class="filters subfilters" id="subcategory-filters" role="group" aria-label="进一步按文件格式筛选"></div></div></div>
<div class="batch-toolbar"><label class="select-all"><input id="select-all" type="checkbox">全选本页</label><span id="selected-count" aria-live="polite">已选 0 个</span><button id="batch-download" type="button" disabled>批量下载</button><button id="clear-selection" type="button" disabled>取消选择</button></div><p id="batch-status" class="batch-status" role="status" hidden></p>
<div class="list" id="list">{{range .Files}}<div class="file-row"><label class="file-select"><input class="file-checkbox" type="checkbox" aria-label="选择 {{.Name}}"></label><a class="file" data-key="{{.Path}}" data-name="{{.Path}}" data-time="{{.ModifiedUnix}}" data-filters="{{.FilterTags}}" href="/download?{{if $.Protected}}token={{$.AccessToken | urlquery}}&amp;{{end}}path={{.Path | urlquery}}" download="{{.Name}}"><span class="icon {{.Category}}" data-icon="{{.Icon}}" aria-hidden="true">{{if eq .Icon "pdf"}}<svg viewBox="0 0 24 24" fill="none"><path d="M6 3.5h8l4 4v13H6zM14 3.5v4h4" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M7.7 16.3v-4.5h1.1a1.15 1.15 0 0 1 0 2.3H7.7M10.5 16.3v-4.5h.9c1.5 0 2.2.85 2.2 2.25s-.7 2.25-2.2 2.25zM14.5 16.3v-4.5h2M14.5 14h1.6" stroke="currentColor" stroke-width=".9" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "word"}}<svg viewBox="0 0 24 24" fill="none"><path d="M6 3.5h8l4 4v13H6zM14 3.5v4h4" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="m8.5 11 1.3 5 2.2-3.4 2.2 3.4 1.3-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "spreadsheet"}}<svg viewBox="0 0 24 24" fill="none"><rect x="4" y="4" width="16" height="16" rx="1" stroke="currentColor" stroke-width="1.6"/><path d="M4 9h16M9.5 9v11M4 14.5h16M15 9v11" stroke="currentColor" stroke-width="1.35"/></svg>{{else if eq .Icon "presentation"}}<svg viewBox="0 0 24 24" fill="none"><path d="M4 4.5h16v11H4zM12 15.5v4M8.5 19.5h7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><path d="m8 12 2.5-2.5 2 1.7L16 7.8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "text"}}<svg viewBox="0 0 24 24" fill="none"><path d="M6 3.5h8l4 4v13H6zM14 3.5v4h4M8.5 11h7M8.5 14h7M8.5 17h5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "markdown"}}<svg viewBox="0 0 24 24" fill="none"><rect x="3.5" y="5" width="17" height="14" rx="1.5" stroke="currentColor" stroke-width="1.6"/><path d="M6.5 15v-6l2.5 3 2.5-3v6M15 9v6m-2-2 2 2 2-2" stroke="currentColor" stroke-width="1.55" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "code"}}<svg viewBox="0 0 24 24" fill="none"><path d="m9 7-5 5 5 5M15 7l5 5-5 5M13.5 4.5l-3 15" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "image"}}<svg viewBox="0 0 24 24" fill="none"><rect x="3.5" y="4.5" width="17" height="15" rx="1.5" stroke="currentColor" stroke-width="1.7"/><circle cx="9" cy="9.5" r="1.5" fill="currentColor"/><path d="m5.5 17 4-4 3 2.8 2.2-2.3 3.8 3.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "video"}}<svg viewBox="0 0 24 24" fill="none"><rect x="3.5" y="5" width="17" height="14" rx="1.5" stroke="currentColor" stroke-width="1.7"/><path d="m10 9 5 3-5 3z" fill="currentColor"/></svg>{{else if eq .Icon "audio"}}<svg viewBox="0 0 24 24" fill="none"><path d="M9.5 17V6.5l8-2v10.7M9.5 9l8-2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><circle cx="7" cy="17" r="2.5" stroke="currentColor" stroke-width="1.7"/><circle cx="15" cy="15.5" r="2.5" stroke="currentColor" stroke-width="1.7"/></svg>{{else if eq .Icon "ebook"}}<svg viewBox="0 0 24 24" fill="none"><path d="M4 5.5c3.2-.7 5.9.1 8 2.2v12c-2.1-2.1-4.8-2.9-8-2.2zM20 5.5c-3.2-.7-5.9.1-8 2.2v12c2.1-2.1 4.8-2.9 8-2.2z" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "archive"}}<svg viewBox="0 0 24 24" fill="none"><path d="M5 8h14v11.5H5zM4 4.5h16V8H4zM10 12h4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><path d="M10 4.5h4" stroke="currentColor" stroke-width="1.7"/></svg>{{else if eq .Icon "windows"}}<svg viewBox="0 0 24 24" fill="currentColor"><path d="m3.5 5.5 7.5-1v7H3.5zm8.5-1.2 8.5-1.2v8.4H12zM3.5 12.5H11v7l-7.5-1zm8.5 0h8.5v8.4L12 19.7z"/></svg>{{else if eq .Icon "macos"}}<svg viewBox="0 0 24 24" fill="none"><path d="M7 8.5 12 5l5 3.5v7L12 19l-5-3.5zM7 8.5l5 3.5 5-3.5M12 12v7M12 5v-2" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "linux"}}<svg viewBox="0 0 24 24" fill="none"><rect x="3.5" y="5" width="17" height="14" rx="1.5" stroke="currentColor" stroke-width="1.6"/><path d="m7 9 3 3-3 3M12 15h5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "android"}}<svg viewBox="0 0 24 24" fill="none"><path d="M6 10h12v8.5H6zM8 10a4 4 0 0 1 8 0M8 6 6.5 4M16 6l1.5-2M9 14h.01M15 14h.01" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "ios"}}<svg viewBox="0 0 24 24" fill="none"><rect x="7" y="2.5" width="10" height="19" rx="2" stroke="currentColor" stroke-width="1.6"/><path d="M10.5 5h3M11 18.5h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>{{else if eq .Icon "disk-image"}}<svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="8.5" stroke="currentColor" stroke-width="1.6"/><circle cx="12" cy="12" r="2.5" stroke="currentColor" stroke-width="1.6"/><path d="M12 3.5V8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>{{else if eq .Icon "font"}}<svg viewBox="0 0 24 24" fill="none"><path d="m5 19 7-15 7 15M7.5 14h9" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Icon "design"}}<svg viewBox="0 0 24 24" fill="none"><path d="m12 3 6.5 6.5L12 21 5.5 9.5zM5.5 9.5H10M14 9.5h4.5M12 21v-7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><circle cx="12" cy="11.5" r="2.2" stroke="currentColor" stroke-width="1.5"/></svg>{{else if eq .Icon "database"}}<svg viewBox="0 0 24 24" fill="none"><ellipse cx="12" cy="5.5" rx="7.5" ry="3" stroke="currentColor" stroke-width="1.6"/><path d="M4.5 5.5v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6M4.5 11.5v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6" stroke="currentColor" stroke-width="1.6"/></svg>{{else}}<svg viewBox="0 0 24 24" fill="none"><path d="M6.5 3.5h7l4 4v13h-11zM13.5 3.5v4h4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><circle cx="9" cy="14" r="1" fill="currentColor"/><circle cx="12" cy="14" r="1" fill="currentColor"/><circle cx="15" cy="14" r="1" fill="currentColor"/></svg>{{end}}</span><span class="details"><span class="name">{{.Name}}</span><span class="meta">{{if .Folder}}{{.Folder}} · {{end}}{{.Size}} · {{.Modified}}</span></span><span class="download">下载</span></a></div>{{else}}<div class="empty">共享目录中还没有文件</div>{{end}}<div id="filter-empty" class="empty filter-empty" hidden>没有符合当前条件的文件</div></div>
<p class="note">方序传文件 · 文件只在你的局域网内流转，不会上传到云端。请勿将访问地址分享给不信任的人。</p>
</main><script>
const search=document.querySelector('#search'),items=[...document.querySelectorAll('.file')],visible=document.querySelector('#visible'),list=document.querySelector('#list'),sortSelect=document.querySelector('#sort'),categoryFilters=document.querySelector('#category-filters'),subcategoryLevel=document.querySelector('#subcategory-level'),subcategoryFilters=document.querySelector('#subcategory-filters'),filterEmpty=document.querySelector('#filter-empty');
const normalized=value=>value.normalize('NFKC').toLocaleLowerCase();
const categoryDefinitions=[['document','文档'],['image','图片'],['video','视频'],['audio','音频'],['ebook','电子书'],['archive','压缩包'],['installer','安装包'],['other','其他']];
const subcategoryDefinitions={document:[['pdf','PDF'],['word','Word'],['spreadsheet','表格'],['presentation','演示文稿'],['text','文本'],['code','代码与数据'],['other','其他文档']],image:[['jpeg','JPEG'],['png','PNG'],['gif','GIF'],['webp','WebP'],['heic','HEIC / HEIF'],['svg','SVG'],['other','其他图片']],video:[['mp4','MP4'],['mov','MOV'],['mkv','MKV'],['webm','WebM'],['avi','AVI'],['other','其他视频']],audio:[['mp3','MP3'],['aac','M4A / AAC'],['flac','FLAC'],['wav','WAV'],['ogg','OGG / Opus'],['other','其他音频']],ebook:[['pdf','PDF'],['epub','EPUB'],['kindle','Kindle'],['word','Word'],['text','文本'],['web','网页文档'],['fb2','FB2'],['djvu','DJVU'],['comic','漫画书'],['other','其他电子书']],archive:[['zip','ZIP'],['rar','RAR'],['7z','7Z'],['tar','TAR'],['compressed','GZ / BZ2 / XZ'],['other','其他压缩包']],installer:[['windows','Windows'],['macos','macOS'],['linux','Linux'],['android','Android'],['ios','iOS'],['other','其他安装包']],other:[['disk-image','系统镜像'],['font','字体'],['design','设计文件'],['database','数据库'],['unclassified','未分类']]};
const itemTags=item=>item.dataset.filters.split(' '),tagCount=tag=>items.reduce((count,item)=>count+(itemTags(item).includes(tag)?1:0),0);
let activeCategory='all',activeSubcategory='all';
const makeFilterButton=(label,value,count,active,onClick)=>{const button=document.createElement('button');button.className='filter'+(active?' active':'');button.type='button';button.dataset.filter=value;button.setAttribute('aria-pressed',active);const text=document.createElement('span');text.textContent=label;const badge=document.createElement('span');badge.className='filter-count';badge.textContent=count;button.append(text,badge);button.addEventListener('click',onClick);return button};
const renderSubcategories=()=>{subcategoryFilters.replaceChildren();if(activeCategory==='all'){subcategoryLevel.hidden=true;return}const definitions=(subcategoryDefinitions[activeCategory]||[]).filter(([key])=>tagCount(activeCategory+':'+key)>0);subcategoryLevel.hidden=definitions.length===0;if(subcategoryLevel.hidden)return;subcategoryFilters.append(makeFilterButton('全部'+categoryDefinitions.find(([key])=>key===activeCategory)[1],'all',tagCount(activeCategory),activeSubcategory==='all',()=>{activeSubcategory='all';renderSubcategories();filterFiles()}));definitions.forEach(([key,label])=>subcategoryFilters.append(makeFilterButton(label,key,tagCount(activeCategory+':'+key),activeSubcategory===key,()=>{activeSubcategory=key;renderSubcategories();filterFiles()})))};
const renderCategories=()=>{categoryFilters.replaceChildren();categoryFilters.append(makeFilterButton('全部','all',items.length,activeCategory==='all',()=>{activeCategory='all';activeSubcategory='all';renderCategories();renderSubcategories();filterFiles()}));categoryDefinitions.filter(([key])=>tagCount(key)>0).forEach(([key,label])=>categoryFilters.append(makeFilterButton(label,key,tagCount(key),activeCategory===key,()=>{activeCategory=key;activeSubcategory='all';renderCategories();renderSubcategories();filterFiles()})))};
const selectAll=document.querySelector('#select-all'),batchDownload=document.querySelector('#batch-download'),clearSelection=document.querySelector('#clear-selection'),selectedCount=document.querySelector('#selected-count'),checkboxes=[...document.querySelectorAll('.file-checkbox')];
let batchRunning=false;
const updateSelection=()=>{const shown=items.filter(item=>!item.classList.contains('is-hidden')).map(item=>item.parentElement.querySelector('.file-checkbox')),selected=checkboxes.filter(box=>box.checked).length,shownSelected=shown.filter(box=>box.checked).length;selectAll.disabled=shown.length===0;selectAll.checked=shown.length>0&&shownSelected===shown.length;selectAll.indeterminate=shownSelected>0&&shownSelected<shown.length;selectedCount.textContent='已选 '+selected+' 个';batchDownload.disabled=batchRunning||selected===0;clearSelection.disabled=selected===0};
checkboxes.forEach(box=>box.addEventListener('change',updateSelection));selectAll.addEventListener('change',()=>{items.filter(item=>!item.classList.contains('is-hidden')).forEach(item=>item.parentElement.querySelector('.file-checkbox').checked=selectAll.checked);updateSelection()});clearSelection.addEventListener('click',()=>{checkboxes.forEach(box=>box.checked=false);updateSelection()});updateSelection();
batchDownload.addEventListener('click',async()=>{
  if(batchRunning)return;
  const selected=items.filter(item=>item.parentElement.querySelector('.file-checkbox').checked);
  if(selected.length===0)return;
  const status=document.querySelector('#batch-status');
  batchRunning=true;updateSelection();status.hidden=false;
  try{
    for(let i=0;i<selected.length;i++){
      const link=document.createElement('a');link.href=selected[i].href;link.download=selected[i].getAttribute('download');link.hidden=true;document.body.append(link);link.click();link.remove();
      batchDownload.textContent='正在发起 '+(i+1)+' / '+selected.length;
      status.textContent='已发起 '+(i+1)+' / '+selected.length+' 个文件下载。若浏览器提示，请允许下载多个文件。';
      if(i+1<selected.length)await new Promise(resolve=>setTimeout(resolve,500));
    }
    status.textContent='已发起 '+selected.length+' 个文件下载，请在浏览器下载列表查看。若被拦截，请允许下载多个文件后重试。';
  }finally{batchRunning=false;batchDownload.textContent='批量下载';updateSelection()}
});
const filterFiles=()=>{const q=normalized(search.value.trim()),categoryTag=activeSubcategory==='all'?activeCategory:activeCategory+':'+activeSubcategory;let n=0;items.forEach(el=>{const matchesSearch=!q||normalized(el.dataset.key).includes(q),matchesCategory=activeCategory==='all'||itemTags(el).includes(categoryTag),show=matchesSearch&&matchesCategory;el.classList.toggle('is-hidden',!show);el.parentElement.hidden=!show;if(show)n++});visible.textContent=n;filterEmpty.hidden=n>0||items.length===0;updateSelection()};
search.addEventListener('input',filterFiles);search.addEventListener('search',filterFiles);
renderCategories();renderSubcategories();
const compareName=(a,b)=>a.dataset.name.localeCompare(b.dataset.name,undefined,{numeric:true,sensitivity:'base'}),compareTime=(a,b)=>Number(a.dataset.time)-Number(b.dataset.time);sortSelect.addEventListener('change',()=>{const mode=sortSelect.value;items.sort((a,b)=>{switch(mode){case'time-asc':return compareTime(a,b)||compareName(a,b);case'name-asc':return compareName(a,b);case'name-desc':return compareName(b,a);default:return compareTime(b,a)||compareName(a,b)}}).forEach(item=>list.insertBefore(item.parentElement,filterEmpty))});
document.querySelectorAll('[data-copy]').forEach(button=>button.addEventListener('click',async()=>{await navigator.clipboard.writeText(button.dataset.copy);const old=button.textContent;button.textContent='已复制';setTimeout(()=>button.textContent=old,1200)}));
const uploadFiles=document.querySelector('#upload-files'),uploadSelection=document.querySelector('#upload-selection'),uploadForm=document.querySelector('.upload-form'),uploadSubmit=document.querySelector('.upload-submit');uploadFiles.addEventListener('change',()=>{const files=[...uploadFiles.files];uploadSelection.textContent=files.length===0?'尚未选择':files.length===1?files[0].name:'已选择 '+files.length+' 个文件'});uploadForm.addEventListener('submit',event=>{if(uploadFiles.files.length===0){event.preventDefault();uploadFiles.click();return}uploadSubmit.disabled=true;uploadSubmit.textContent='正在上传…'});
</script></body></html>`
