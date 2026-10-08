package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func uploadRequest(t *testing.T, s *server, id string, names ...string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range names {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, "photo data"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/upload?upload_token="+s.token, &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Accept", "application/json")
	if id != "" {
		r.Header.Set("X-Upload-ID", id)
	}
	return r
}

func TestUploadJSONReceiptAndRetryDoNotDuplicatePhotos(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "照片.HEIC"), "existing")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)
	id := strings.Repeat("a", 32)
	var first string
	for attempt := 0; attempt < 2; attempt++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, uploadRequest(t, s, id, "照片.HEIC"))
		if response.Code != http.StatusOK || response.Header().Get("Location") != "" {
			t.Fatalf("unexpected receipt: %d %s", response.Code, response.Body.String())
		}
		var result struct {
			Status string
			Files  []string
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "uploaded" || len(result.Files) != 1 || result.Files[0] != "照片 (1).HEIC" {
			t.Fatalf("wrong saved filename: %+v", result)
		}
		if attempt == 0 {
			first = response.Body.String()
		} else if response.Body.String() != first {
			t.Fatal("retry changed receipt")
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("retry created extra files: %v %v", entries, err)
	}
	secondRoot := t.TempDir()
	if err := s.setRoot(secondRoot); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, uploadRequest(t, s, id, "照片.HEIC"))
	if response.Code != http.StatusOK {
		t.Fatalf("upload into changed directory: %s", response.Body.String())
	}
	if data, err := os.ReadFile(filepath.Join(secondRoot, "照片.HEIC")); err != nil || string(data) != "photo data" {
		t.Fatalf("old receipt skipped saving in new directory: %q %v", data, err)
	}
}

func TestConcurrentUploadRetriesCommitOnlyOnce(t *testing.T) {
	root := t.TempDir()
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)
	requests := make([]*http.Request, 8)
	responses := make([]*httptest.ResponseRecorder, len(requests))
	for i := range requests {
		requests[i] = uploadRequest(t, s, strings.Repeat("b", 32), "旅行.jpg")
		responses[i] = httptest.NewRecorder()
	}
	var group sync.WaitGroup
	for i := range requests {
		group.Add(1)
		go func(i int) { defer group.Done(); handler.ServeHTTP(responses[i], requests[i]) }(i)
	}
	group.Wait()
	for _, response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("retry failed: %d %s", response.Code, response.Body.String())
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "旅行.jpg" {
		t.Fatalf("duplicate or temporary files: %v %v", entries, err)
	}
}

func TestUploadJSONErrorsPreserveNoPartialFilesAndCanRetry(t *testing.T) {
	root := t.TempDir()
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.routes(8080)
	id := strings.Repeat("c", 32)
	tooMany := make([]string, maxUploadFiles+1)
	for i := range tooMany {
		tooMany[i] = "photo-" + strconv.Itoa(i) + ".jpg"
	}
	for _, tc := range []struct {
		name, id string
		names    []string
		status   string
	}{
		{"invalid name after valid photo", id, []string{"ok.jpg", ".hidden.jpg"}, "upload-invalid"},
		{"empty selection", id, nil, "upload-empty"},
		{"invalid request ID", "BAD", []string{"ok.jpg"}, "upload-invalid"},
		{"too many photos", id, tooMany, "upload-too-many"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, uploadRequest(t, s, tc.id, tc.names...))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tc.status) {
				t.Fatalf("wrong error: %d %s", response.Code, response.Body.String())
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatalf("partial upload left behind: %v", entries)
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, uploadRequest(t, s, id, "ok.jpg"))
	if response.Code != http.StatusOK {
		t.Fatalf("failed request prevented retry: %s", response.Body.String())
	}
}

func TestUploadJSONRetryStillRequiresValidAccessTokens(t *testing.T) {
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.setProtected(true)
	handler := s.routes(8080)
	id := strings.Repeat("d", 32)
	request := uploadRequest(t, s, id, "ok.jpg")
	request.URL.RawQuery += "&token=" + s.token
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("protected upload: %d", response.Code)
	}
	request = uploadRequest(t, s, id, "ok.jpg")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "upload-unauthorized") {
		t.Fatalf("retry bypassed protection: %d %s", response.Code, response.Body.String())
	}
}

func TestUploadLargePhotoSelectionAndRetry(t *testing.T) {
	root := t.TempDir()
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, maxUploadFiles)
	for i := range names {
		names[i] = "holiday-" + strconv.Itoa(i) + ".HEIC"
	}
	handler := s.routes(8080)
	id := strings.Repeat("e", 32)
	for attempt := 0; attempt < 2; attempt++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, uploadRequest(t, s, id, names...))
		var result struct {
			Status string
			Files  []string
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || result.Status != "uploaded" || len(result.Files) != len(names) {
			t.Fatalf("large photo selection: status=%d saved=%d", response.Code, len(result.Files))
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(names) {
		t.Fatalf("large selection lost or duplicated files: count=%d error=%v", len(entries), err)
	}
	for _, name := range []string{names[0], names[len(names)-1]} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != "photo data" {
			t.Fatalf("photo contents: %q %v", data, err)
		}
	}
}
