package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type relayRequest struct {
	Provider string
	Path     string
	Size     int64
}

type relayProgress struct {
	Uploaded int64   `json:"uploaded"`
	Speed    float64 `json:"speed"`
}

type relayResponse struct {
	Progress *relayProgress `json:"progress,omitempty"`
	URL      string         `json:"url,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type relayProgressReader struct {
	io.Reader
	uploaded *atomic.Int64
}

func (r relayProgressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.uploaded.Add(int64(n))
	return n, err
}

func reportRelayProgress(encoder *json.Encoder, uploaded *atomic.Int64, done <-chan struct{}) {
	const interval = 350 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastTime := time.Now()
	var lastBytes int64
	var lastSpeed float64
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			bytes := uploaded.Load()
			if bytes != lastBytes || lastSpeed > 0 {
				speed := float64(bytes-lastBytes) / now.Sub(lastTime).Seconds()
				if err := encoder.Encode(relayResponse{Progress: &relayProgress{Uploaded: bytes, Speed: speed}}); err != nil {
					return
				}
				lastSpeed = speed
			}
			lastBytes, lastTime = bytes, now
		}
	}
}

var (
	errRelayConnection = errors.New("无法连接中转服务，请重试")
	errRelayRejected   = errors.New("中转服务拒绝了上传")
	errRelayResponse   = errors.New("中转服务返回了无效响应")
	errRelayFile       = errors.New("文件无法读取或已修改，请重新选择")
)

func runRelayUpload(input io.Reader, output io.Writer) error {
	var request relayRequest
	if err := json.NewDecoder(io.LimitReader(input, 64<<10)).Decode(&request); err != nil {
		return json.NewEncoder(output).Encode(relayResponse{Error: errRelayFile.Error()})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// The desktop client keeps this pipe open for the lifetime of the upload.
		_, _ = io.Copy(io.Discard, input)
		cancel()
	}()

	encoder := json.NewEncoder(output)
	var uploaded atomic.Int64
	done, flushed := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(flushed)
		reportRelayProgress(encoder, &uploaded, done)
	}()
	link, err := relayUpload(ctx, request, &uploaded)
	close(done)
	<-flushed

	response := relayResponse{}
	if err != nil {
		response.Error = err.Error()
	} else {
		response.URL = link
	}
	return encoder.Encode(response)
}

func relayUpload(ctx context.Context, request relayRequest, uploaded *atomic.Int64) (string, error) {
	switch request.Provider {
	case "kiwi", "gofile", "tempfile", "tmp0", "filebin":
	default:
		return "", errRelayRejected
	}
	file, err := os.Open(request.Path)
	if err != nil {
		return "", errRelayFile
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != request.Size {
		return "", errRelayFile
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 90 * time.Second
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	switch request.Provider {
	case "kiwi":
		return uploadKiwi(ctx, client, file, info.Size(), uploaded)
	case "tempfile":
		return uploadTempFile(ctx, client, file, info.Size(), uploaded)
	case "tmp0":
		return uploadTmp0(ctx, client, file, info.Size(), uploaded)
	case "filebin":
		return uploadFilebin(ctx, client, file, info.Size(), uploaded)
	}
	body, err := uploadRelayMultipart(ctx, client, "https://upload.gofile.io/uploadfile", "file", file, info.Size(), uploaded)
	if err != nil {
		return "", err
	}
	defer body.Close()
	var result struct {
		Status string
		Data   struct{ DownloadPage string }
	}
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&result); err != nil {
		return "", errRelayResponse
	}
	if result.Status != "ok" {
		return "", errRelayRejected
	}
	if !validRelayURL(result.Data.DownloadPage, "gofile.io") {
		return "", errRelayResponse
	}
	return result.Data.DownloadPage, nil
}

func uploadTempFile(ctx context.Context, client *http.Client, file *os.File, size int64, uploaded *atomic.Int64) (string, error) {
	if size > 100*1000*1000 {
		return "", errors.New("TempFile.org 单个文件最大 100 MB")
	}
	body, err := uploadRelayMultipart(ctx, client, "https://tempfile.org/api/upload/local", "files[]", file, size, uploaded)
	if err != nil {
		return "", err
	}
	defer body.Close()
	var result struct {
		Success bool
		Files   []struct{ URL string }
	}
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&result); err != nil {
		return "", errRelayResponse
	}
	if !result.Success {
		return "", errRelayRejected
	}
	if len(result.Files) != 1 || !validRelayURL(result.Files[0].URL, "tempfile.org") {
		return "", errRelayResponse
	}
	return result.Files[0].URL, nil
}

func uploadTmp0(ctx context.Context, client *http.Client, file *os.File, size int64, uploaded *atomic.Int64) (string, error) {
	if size > 2*1000*1000*1000 {
		return "", errors.New("tmp0.cc 单个文件最大 2 GB")
	}
	body, err := uploadRelayMultipart(ctx, client, "https://tmp0.cc/api/v1/upload", "file", file, size, uploaded)
	if err != nil {
		return "", err
	}
	defer body.Close()
	var result struct {
		Success bool
		FullURL string
	}
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&result); err != nil {
		return "", errRelayResponse
	}
	if !result.Success {
		return "", errRelayRejected
	}
	if !validRelayURL(result.FullURL, "tmp0.cc") {
		return "", errRelayResponse
	}
	return result.FullURL, nil
}

func uploadFilebin(ctx context.Context, client *http.Client, file *os.File, size int64, uploaded *atomic.Int64) (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", errRelayResponse
	}
	id := fmt.Sprintf("%x", token)
	link := "https://filebin.net/" + id
	content := relayProgressReader{Reader: io.NewSectionReader(file, 0, size), uploaded: uploaded}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, link+"/"+url.PathEscape(filepath.Base(file.Name())), content)
	if err != nil {
		return "", errRelayResponse
	}
	request.ContentLength = size
	if size == 0 {
		request.Body = http.NoBody
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	body, err := sendRelayRequest(client, request)
	if err != nil {
		return "", err
	}
	defer body.Close()
	var result struct {
		Bin  struct{ ID string }
		File struct{ Bytes int64 }
	}
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&result); err != nil || result.Bin.ID != id || result.File.Bytes != size {
		return "", errRelayResponse
	}
	return link, nil
}

func uploadRelayMultipart(ctx context.Context, client *http.Client, endpoint, field string, file *os.File, size int64, uploaded *atomic.Int64) (io.ReadCloser, error) {
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	if _, err := writer.CreateFormFile(field, filepath.Base(file.Name())); err != nil {
		return nil, errRelayFile
	}
	offset := form.Len()
	if err := writer.Close(); err != nil {
		return nil, errRelayFile
	}
	content := relayProgressReader{Reader: io.NewSectionReader(file, 0, size), uploaded: uploaded}
	body := io.MultiReader(bytes.NewReader(form.Bytes()[:offset]), content, bytes.NewReader(form.Bytes()[offset:]))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, errRelayResponse
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.ContentLength = int64(form.Len()) + size
	return sendRelayRequest(client, request)
}

func sendRelayRequest(client *http.Client, request *http.Request) (io.ReadCloser, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, errRelayConnection
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, errRelayRejected
	}
	return response.Body, nil
}

func validRelayURL(raw, host string) bool {
	link, err := url.Parse(raw)
	return err == nil && link.Scheme == "https" && link.User == nil && link.Host == host && len(link.Path) > 1
}

const kiwiAPI = "https://api.file.kiwi/v2"
const kiwiRecordSize = 65536
const kiwiPlainRecordSize = kiwiRecordSize - 17

type kiwiUpload struct {
	FolderID   string
	UploadAuth string
	Files      []struct {
		FileID     string
		ChunkSize  int64
		Chunks     int64
		UploadURLs struct {
			Head       string
			Path       string
			Tail       string
			Signatures []string
			Headers    map[string]string
		}
	}
}

func uploadKiwi(ctx context.Context, client *http.Client, file *os.File, size int64, uploaded *atomic.Int64) (string, error) {
	if size == 0 || size > 999<<30 {
		return "", errors.New("file.kiwi 仅支持大于 0 字节且不超过 999 GiB 的文件")
	}
	name := filepath.Base(file.Name())
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".exe", ".dll", ".com", ".bat", ".cmd", ".msi", ".ps1", ".sh", ".apk", ".app", ".dmg", ".pkg", ".deb", ".rpm", ".scr", ".js", ".vbs", ".jar":
		return "", errors.New("file.kiwi 不支持此类可执行文件")
	}
	contentType := mime.TypeByExtension(ext)
	contentType, _, _ = mime.ParseMediaType(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", errRelayResponse
	}
	secret := base64.RawURLEncoding.EncodeToString(key)
	encrypted := func(value string) (string, error) {
		var buffer bytes.Buffer
		if err := encryptKiwi(&buffer, strings.NewReader(value), int64(len(value)), key); err != nil {
			return "", errRelayResponse
		}
		return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
	}
	ske, err := encrypted(secret)
	if err != nil {
		return "", err
	}
	filename, err := encrypted(name)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"mode":       "send",
		"encryption": map[string]string{"ske": ske},
		"files":      []map[string]any{{"filename": filename, "filesize": size, "mimetype": contentType}},
	})
	if err != nil {
		return "", errRelayResponse
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, kiwiAPI+"/folders", bytes.NewReader(payload))
	if err != nil {
		return "", errRelayResponse
	}
	request.Header.Set("Content-Type", "application/json")
	body, err := sendRelayRequest(client, request)
	if err != nil {
		return "", err
	}
	var session kiwiUpload
	err = json.NewDecoder(io.LimitReader(body, 16<<20)).Decode(&session)
	body.Close()
	if err != nil || len(session.Files) != 1 || !kiwiID(session.FolderID) || session.UploadAuth == "" {
		return "", errRelayResponse
	}
	item := session.Files[0]
	if !kiwiID(item.FileID) || item.ChunkSize < 1 || item.Chunks != (size-1)/item.ChunkSize+1 || int64(len(item.UploadURLs.Signatures)) != item.Chunks {
		return "", errRelayResponse
	}
	start := time.Now()
	statusURL := fmt.Sprintf("%s/folders/%s/files/%s/upload-status?uploadAuth=%s", kiwiAPI, session.FolderID, item.FileID, url.QueryEscape(session.UploadAuth))
	for index, signature := range item.UploadURLs.Signatures {
		offset := int64(index) * item.ChunkSize
		length := min(item.ChunkSize, size-offset)
		target := fmt.Sprintf("%s%s/%05d?%s&X-Amz-Signature=%s", item.UploadURLs.Head, item.UploadURLs.Path, index+1, item.UploadURLs.Tail, url.QueryEscape(signature))
		signed, err := url.Parse(target)
		if err != nil || signed.Scheme != "https" || signed.User != nil || signed.Port() != "" || !strings.HasSuffix(signed.Hostname(), ".r2.cloudflarestorage.com") {
			return "", errRelayResponse
		}
		content := relayProgressReader{Reader: io.NewSectionReader(file, offset, length), uploaded: uploaded}
		if err := uploadKiwiChunk(ctx, client, target, item.UploadURLs.Headers, content, length, key); err != nil {
			return "", err
		}
		if time.Since(start) > 15*time.Minute && index+1 < len(item.UploadURLs.Signatures) {
			if _, err := kiwiComplete(ctx, client, statusURL); err != nil {
				return "", err
			}
			start = time.Now()
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		complete, err := kiwiComplete(ctx, client, statusURL)
		if err != nil {
			return "", err
		}
		if complete {
			return "https://file.kiwi/" + session.FolderID + "#" + secret, nil
		}
		select {
		case <-ctx.Done():
			return "", errRelayConnection
		case <-time.After(time.Second):
		}
	}
	return "", errRelayResponse
}

func kiwiID(id string) bool {
	if id == "" {
		return false
	}
	for _, ch := range id {
		if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-", ch) {
			return false
		}
	}
	return true
}

func kiwiComplete(ctx context.Context, client *http.Client, endpoint string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, errRelayResponse
	}
	body, err := sendRelayRequest(client, request)
	if err != nil {
		return false, err
	}
	defer body.Close()
	var status struct{ Complete bool }
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&status); err != nil {
		return false, errRelayResponse
	}
	return status.Complete, nil
}

func uploadKiwiChunk(ctx context.Context, client *http.Client, target string, headers map[string]string, file io.Reader, size int64, key []byte) error {
	reader, writer := io.Pipe()
	defer reader.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target, reader)
	if err != nil {
		writer.Close()
		return errRelayResponse
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.ContentLength = 21 + size + 17*((size-1)/kiwiPlainRecordSize+1)
	go func() { _ = writer.CloseWithError(encryptKiwi(writer, file, size, key)) }()
	body, err := sendRelayRequest(client, request)
	if err != nil {
		return err
	}
	return body.Close()
}

// file.kiwi independently encrypts the metadata and each upload chunk using RFC 8188.
func encryptKiwi(dst io.Writer, src io.Reader, size int64, key []byte) error {
	header := make([]byte, 21)
	if _, err := rand.Read(header[:16]); err != nil {
		return err
	}
	binary.BigEndian.PutUint32(header[16:20], kiwiRecordSize)
	extract := hmac.New(sha256.New, header[:16])
	extract.Write(key)
	prk := extract.Sum(nil)
	expand := func(label string, length int) []byte {
		mac := hmac.New(sha256.New, prk)
		mac.Write([]byte(label + "\x00\x01"))
		return mac.Sum(nil)[:length]
	}
	block, err := aes.NewCipher(expand("Content-Encoding: aes128gcm", 16))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	baseNonce := expand("Content-Encoding: nonce", 12)
	if _, err := dst.Write(header); err != nil {
		return err
	}
	buffer := make([]byte, kiwiRecordSize)
	var nonce [12]byte
	for sequence := uint64(0); ; sequence++ {
		length := int(min(size, kiwiPlainRecordSize))
		if _, err := io.ReadFull(src, buffer[:length]); err != nil {
			return err
		}
		size -= int64(length)
		buffer[length] = 1
		if size == 0 {
			buffer[length] = 2
		}
		copy(nonce[:], baseNonce)
		binary.BigEndian.PutUint64(nonce[4:], binary.BigEndian.Uint64(nonce[4:])^sequence)
		encoded := gcm.Seal(buffer[:0], nonce[:], buffer[:length+1], nil)
		if _, err := dst.Write(encoded); err != nil {
			return err
		}
		if size == 0 {
			return nil
		}
	}
}
