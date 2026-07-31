package app

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type baiduRemoteObject struct {
	content []byte
	fsID    string
	md5     string
}

type baiduRewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t baiduRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.target.Scheme
	cloned.URL.Host = t.target.Host
	cloned.Host = t.target.Host
	return base.RoundTrip(cloned)
}

func TestLoadBaiduTokenRejectsUnsafeFiles(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "baiduyun_token.json")
	if err := os.WriteFile(tokenPath, []byte(`{"access_token":"valid-token"}`), 0600); err != nil {
		t.Fatalf("WriteFile token error = %v", err)
	}
	token, err := LoadBaiduToken(tokenPath)
	if err != nil {
		t.Fatalf("LoadBaiduToken(valid) error = %v", err)
	}
	if token.AccessToken != "valid-token" {
		t.Fatalf("expected valid-token, got %q", token.AccessToken)
	}

	oversizedPath := filepath.Join(t.TempDir(), "oversized-token.json")
	oversizedBody := `{"access_token":"` + strings.Repeat("a", int(seedConfigFileLimitBytes)+1) + `"}`
	if err := os.WriteFile(oversizedPath, []byte(oversizedBody), 0600); err != nil {
		t.Fatalf("WriteFile oversized token error = %v", err)
	}
	if _, err := LoadBaiduToken(oversizedPath); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized token read error, got %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "target-token.json")
	if err := os.WriteFile(targetPath, []byte(`{"access_token":"linked-token"}`), 0600); err != nil {
		t.Fatalf("WriteFile linked target token error = %v", err)
	}
	linkPath := filepath.Join(t.TempDir(), "linked-token.json")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("Symlink unavailable: %v", err)
	}
	if _, err := LoadBaiduToken(linkPath); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlink token read error, got %v", err)
	}

	dirPath := filepath.Join(t.TempDir(), "token-dir")
	if err := os.Mkdir(dirPath, 0700); err != nil {
		t.Fatalf("Mkdir token dir error = %v", err)
	}
	if _, err := LoadBaiduToken(dirPath); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected non-regular token read error, got %v", err)
	}
}

func TestBaiduPCSClientRedactsTokenQueryFromNetworkErrors(t *testing.T) {
	const accessToken = "network-access-token"
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: accessToken})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/api/quota" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"errno":0,"expire":0}`)),
				Request:    req,
			}, nil
		}
		return nil, fmt.Errorf("network failed for %s", req.URL.String())
	})}

	sourcePath := filepath.Join(t.TempDir(), "source.db")
	if err := os.WriteFile(sourcePath, []byte("sqlite snapshot"), 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{
			name: "list",
			run: func() error {
				_, err := client.ListFiles(syncRemoteRootPath())
				return err
			},
		},
		{
			name: "create dir",
			run: func() error {
				return client.EnsureRemoteDirectories(path.Join(syncRemoteRootPath(), "papers"))
			},
		},
		{
			name: "delete",
			run: func() error {
				return client.DeleteFile(path.Join(syncRemoteRootPath(), "papers", "paper.pdf"))
			},
		},
		{
			name: "upload",
			run: func() error {
				return client.UploadFileToPath(sourcePath, path.Join(syncRemoteRootPath(), "data", "diveend.db"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatal("expected network error")
			}
			message := err.Error()
			if strings.Contains(message, accessToken) {
				t.Fatalf("expected access token to be redacted from %q", message)
			}
			if strings.Contains(message, "access_token="+accessToken) {
				t.Fatalf("expected raw access_token query to be redacted from %q", message)
			}
			if !strings.Contains(message, "access_token="+redactedValue) {
				t.Fatalf("expected redacted access_token marker in %q", message)
			}
		})
	}
}

func TestBaiduRefreshTokenRedactsCredentialQueryFromNetworkError(t *testing.T) {
	token := &BaiduToken{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network failed for %s", req.URL.String())
	})}

	_, err := refreshTokenWithClient(client, token)
	if err == nil {
		t.Fatal("expected refresh network error")
	}
	message := err.Error()
	for _, leaked := range []string{"refresh-secret", "client-secret"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, message)
		}
	}
	if !strings.Contains(message, "refresh_token="+redactedValue) {
		t.Fatalf("expected redacted refresh token marker in %q", message)
	}
	if !strings.Contains(message, "client_secret="+redactedValue) {
		t.Fatalf("expected redacted client secret marker in %q", message)
	}
}

func TestBaiduPCSClientRedactsDecodedFailureMaps(t *testing.T) {
	const (
		accessToken  = "decoded-access-secret"
		refreshToken = "decoded-refresh-secret"
		clientSecret = "decoded-client-secret"
		bearerToken  = "decoded-bearer-secret"
		apiKey       = "sk-decodedsecret"
	)

	sensitivePayload := func() map[string]any {
		return map[string]any{
			"errno":          -6,
			"access_token":   accessToken,
			"refresh_token":  refreshToken,
			"client_secret":  clientSecret,
			"api_key":        apiKey,
			"show_msg":       "failed with access_token=" + accessToken + " refresh_token=" + refreshToken + " client_secret=" + clientSecret + " Authorization: Bearer " + bearerToken,
			"nested_message": "token=" + accessToken,
		}
	}

	jsonResponse := func(req *http.Request, payload map[string]any) (*http.Response, error) {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal response payload error = %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(data)),
			Request:    req,
		}, nil
	}

	newClient := func(fileHandler func(*http.Request) map[string]any, uploadHandler func(*http.Request) map[string]any) *BaiduPCSClient {
		client := NewBaiduPCSClient(&BaiduToken{AccessToken: accessToken})
		client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/api/quota":
				return jsonResponse(req, map[string]any{"errno": 0, "expire": 0})
			case "/rest/2.0/xpan/file":
				return jsonResponse(req, fileHandler(req))
			case "/rest/2.0/pcs/superfile2":
				return jsonResponse(req, uploadHandler(req))
			default:
				return jsonResponse(req, map[string]any{"errno": -404})
			}
		})}
		return client
	}

	sourcePath := filepath.Join(t.TempDir(), "source.db")
	if err := os.WriteFile(sourcePath, []byte("db"), 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	cases := []struct {
		name string
		run  func(*BaiduPCSClient) error
		file func(*http.Request) map[string]any
		part func(*http.Request) map[string]any
	}{
		{
			name: "list",
			run: func(client *BaiduPCSClient) error {
				_, err := client.ListFiles(syncRemoteRootPath())
				return err
			},
			file: func(req *http.Request) map[string]any {
				if req.URL.Query().Get("method") != "list" {
					t.Fatalf("unexpected file method %q", req.URL.Query().Get("method"))
				}
				return sensitivePayload()
			},
		},
		{
			name: "create directory",
			run: func(client *BaiduPCSClient) error {
				return client.CreateDirectory(path.Join(syncRemoteRootPath(), "decoded-error"))
			},
			file: func(req *http.Request) map[string]any {
				if req.URL.Query().Get("method") != "create" {
					t.Fatalf("unexpected file method %q", req.URL.Query().Get("method"))
				}
				return sensitivePayload()
			},
		},
		{
			name: "delete",
			run: func(client *BaiduPCSClient) error {
				return client.DeleteFile(path.Join(syncRemoteRootPath(), "decoded-error.db"))
			},
			file: func(req *http.Request) map[string]any {
				if req.URL.Query().Get("method") != "filemanager" || req.URL.Query().Get("opera") != "delete" {
					t.Fatalf("unexpected delete query method=%q opera=%q", req.URL.Query().Get("method"), req.URL.Query().Get("opera"))
				}
				return sensitivePayload()
			},
		},
		{
			name: "precreate",
			run: func(client *BaiduPCSClient) error {
				return client.UploadFileToPath(sourcePath, path.Join(syncRemoteRootPath(), "precreate-error.db"))
			},
			file: func(req *http.Request) map[string]any {
				switch req.URL.Query().Get("method") {
				case "create":
					if err := req.ParseForm(); err != nil {
						t.Fatalf("ParseForm directory create error = %v", err)
					}
					if req.Form.Get("isdir") == "1" {
						return map[string]any{"errno": 0, "fs_id": 1}
					}
				case "precreate":
					return sensitivePayload()
				}
				if req.URL.Query().Get("method") != "precreate" {
					t.Fatalf("unexpected file method %q", req.URL.Query().Get("method"))
				}
				return sensitivePayload()
			},
		},
		{
			name: "create file",
			run: func(client *BaiduPCSClient) error {
				return client.UploadFileToPath(sourcePath, path.Join(syncRemoteRootPath(), "create-error.db"))
			},
			file: func(req *http.Request) map[string]any {
				switch req.URL.Query().Get("method") {
				case "precreate":
					return map[string]any{"errno": 0, "uploadid": "decoded-upload"}
				case "create":
					if err := req.ParseForm(); err != nil {
						t.Fatalf("ParseForm create error = %v", err)
					}
					if req.Form.Get("isdir") == "1" {
						return map[string]any{"errno": 0, "fs_id": 1}
					}
					return sensitivePayload()
				default:
					t.Fatalf("unexpected file method %q", req.URL.Query().Get("method"))
					return nil
				}
			},
			part: func(req *http.Request) map[string]any {
				return map[string]any{}
			},
		},
	}

	secrets := []string{accessToken, refreshToken, clientSecret, bearerToken, apiKey}
	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			partHandler := tc.part
			if partHandler == nil {
				partHandler = func(req *http.Request) map[string]any {
					t.Fatalf("unexpected upload request %s", req.URL.String())
					return nil
				}
			}
			err := tc.run(newClient(tc.file, partHandler))
			if err == nil {
				t.Fatal("expected decoded failure map error")
			}
			combined := err.Error() + "\n" + logs.String()
			for _, leaked := range secrets {
				if strings.Contains(combined, leaked) {
					t.Fatalf("expected %q to be redacted from %q", leaked, combined)
				}
			}
			if !strings.Contains(combined, redactedValue) {
				t.Fatalf("expected redacted marker in %q", combined)
			}
		})
	}
}

func TestBaiduPCSClientUploadListAndDownloadContracts(t *testing.T) {
	var (
		createdDirs    []string
		uploadChunks   = map[int][]byte{}
		remoteObjects  = map[string]baiduRemoteObject{}
		fsIDToPath     = map[string]string{}
		quotaChecks    int
		uploadID       = "upload-contract-test"
		nextFSID       = 1000
		uploadedPath   = path.Join(syncRemoteRootPath(), "data", "diveend.db")
		downloadTarget = filepath.Join(t.TempDir(), "downloaded.db")
	)

	nextID := func() string {
		nextFSID++
		return fmt.Sprintf("%d", nextFSID)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}

		switch r.URL.Path {
		case "/api/quota":
			quotaChecks++
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
			return
		case "/rest/2.0/xpan/file":
			handleBaiduFileContract(t, w, r, uploadedPath, uploadID, &createdDirs, uploadChunks, remoteObjects, fsIDToPath, nextID)
			return
		case "/rest/2.0/pcs/superfile2":
			handleBaiduUploadContract(t, w, r, uploadedPath, uploadID, uploadChunks)
			return
		case "/rest/2.0/xpan/multimedia":
			handleBaiduMetaContract(t, w, r, fsIDToPath)
			return
		default:
			if strings.HasPrefix(r.URL.Path, "/download/") {
				fsID := strings.TrimPrefix(r.URL.Path, "/download/")
				remotePath := fsIDToPath[fsID]
				object, ok := remoteObjects[remotePath]
				if !ok {
					http.NotFound(w, r)
					return
				}
				if got := r.Header.Get("User-Agent"); got != "pan.baidu.com" {
					t.Fatalf("expected Baidu download User-Agent, got %q", got)
				}
				_, _ = w.Write(object.content)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	sourcePath := filepath.Join(t.TempDir(), "diveend.db")
	sourceContent := append(bytes.Repeat([]byte("a"), blockSize), []byte("tail")...)
	if err := os.WriteFile(sourcePath, sourceContent, 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	if err := client.UploadFileToPath(sourcePath, uploadedPath); err != nil {
		t.Fatalf("UploadFileToPath() error = %v", err)
	}

	expectedDirs := []string{
		path.Join("/apps", syncApp, syncDataRootName),
		path.Join("/apps", syncApp, syncDataRootName, "data"),
	}
	if !stringSlicesEqual(createdDirs, expectedDirs) {
		t.Fatalf("unexpected created dirs: got %#v want %#v", createdDirs, expectedDirs)
	}
	if quotaChecks == 0 {
		t.Fatal("expected token quota checks to be performed")
	}
	if len(uploadChunks) != 2 {
		t.Fatalf("expected 2 uploaded chunks, got %d", len(uploadChunks))
	}
	uploadedObject, ok := remoteObjects[uploadedPath]
	if !ok {
		t.Fatalf("expected uploaded object at %s", uploadedPath)
	}
	if !bytes.Equal(uploadedObject.content, sourceContent) {
		t.Fatal("uploaded content does not match source content")
	}

	files, err := client.ListFilesRecursive(syncRemoteRootPath())
	if err != nil {
		t.Fatalf("ListFilesRecursive() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != uploadedPath || files[0].Size != int64(len(sourceContent)) {
		t.Fatalf("unexpected recursive file list: %+v", files)
	}

	if err := client.DownloadFile(uploadedPath, downloadTarget); err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	downloaded, err := os.ReadFile(downloadTarget)
	if err != nil {
		t.Fatalf("ReadFile downloaded error = %v", err)
	}
	if !bytes.Equal(downloaded, sourceContent) {
		t.Fatal("downloaded content does not match uploaded content")
	}
	downloadInfo, err := os.Stat(downloadTarget)
	if err != nil {
		t.Fatalf("Stat downloaded file error = %v", err)
	}
	if downloadInfo.Mode().Perm() != 0600 {
		t.Fatalf("expected downloaded file permissions 0600, got %o", downloadInfo.Mode().Perm())
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(downloadTarget), "."+filepath.Base(downloadTarget)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp download files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp download files, got %v", tempMatches)
	}
}

func TestBaiduPCSClientUploadLogsDoNotExposeSensitivePathsOrIDs(t *testing.T) {
	var (
		createdDirs   []string
		uploadChunks  = map[int][]byte{}
		remoteObjects = map[string]baiduRemoteObject{}
		fsIDToPath    = map[string]string{}
		uploadID      = "sensitive-upload-id"
		nextFSID      = 2000
		uploadedPath  = path.Join(syncRemoteRootPath(), "papers", "paper-id", "secret-paper.pdf")
	)

	nextID := func() string {
		nextFSID++
		return fmt.Sprintf("%d", nextFSID)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
			return
		case "/rest/2.0/xpan/file":
			handleBaiduFileContract(t, w, r, uploadedPath, uploadID, &createdDirs, uploadChunks, remoteObjects, fsIDToPath, nextID)
			return
		case "/rest/2.0/pcs/superfile2":
			handleBaiduUploadContract(t, w, r, uploadedPath, uploadID, uploadChunks)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	sourcePath := filepath.Join(t.TempDir(), "secret-local-name.pdf")
	sourceContent := append(bytes.Repeat([]byte("s"), blockSize), []byte("tail")...)
	if err := os.WriteFile(sourcePath, sourceContent, 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	if err := client.UploadFileToPath(sourcePath, uploadedPath); err != nil {
		t.Fatalf("UploadFileToPath() error = %v", err)
	}

	output := logs.String()
	for _, forbidden := range []string{sourcePath, uploadedPath, uploadID} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("upload logs exposed sensitive value %q in %q", forbidden, output)
		}
	}
	for _, hash := range blockMD5s(sourceContent) {
		if strings.Contains(output, hash) {
			t.Fatalf("upload logs exposed chunk md5 %q in %q", hash, output)
		}
	}
}

func TestBaiduPCSClientUploadRejectsUnsafeLocalSourcesBeforeRequest(t *testing.T) {
	networkCalled := false
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	tempDir := t.TempDir()
	emptyPath := filepath.Join(tempDir, "empty.db")
	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatalf("WriteFile empty source error = %v", err)
	}
	dirPath := filepath.Join(tempDir, "dir.db")
	if err := os.Mkdir(dirPath, 0700); err != nil {
		t.Fatalf("Mkdir dir source error = %v", err)
	}
	targetPath := filepath.Join(tempDir, "target.db")
	if err := os.WriteFile(targetPath, []byte("db"), 0600); err != nil {
		t.Fatalf("WriteFile symlink target error = %v", err)
	}
	linkPath := filepath.Join(tempDir, "linked.db")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cases := []struct {
		name        string
		sourcePath  string
		wantMessage string
	}{
		{name: "empty file", sourcePath: emptyPath, wantMessage: "empty or invalid"},
		{name: "directory", sourcePath: dirPath, wantMessage: "empty or invalid"},
		{name: "symlink", sourcePath: linkPath, wantMessage: "symbolic link"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			networkCalled = false
			err := client.UploadFileToPath(tc.sourcePath, path.Join(syncRemoteRootPath(), "data", "unsafe.db"))
			if err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q error, got %v", tc.wantMessage, err)
			}
			if networkCalled {
				t.Fatal("Baidu API should not be called for unsafe local upload source")
			}
		})
	}
}

func TestBaiduPCSClientUploadRejectsRemotePathEscapesBeforeRequest(t *testing.T) {
	networkCalled := false
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	sourcePath := filepath.Join(t.TempDir(), "source.db")
	if err := os.WriteFile(sourcePath, []byte("db"), 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	cases := []struct {
		name       string
		uploadFunc func() error
	}{
		{
			name: "upload file path escapes app root",
			uploadFunc: func() error {
				return client.UploadFileToPath(sourcePath, path.Join("/apps", syncApp, "..", "other-app", "file.db"))
			},
		},
		{
			name: "upload file path uses sibling app prefix",
			uploadFunc: func() error {
				return client.UploadFileToPath(sourcePath, path.Join("/apps", syncApp+"-evil", "file.db"))
			},
		},
		{
			name: "upload file path outside apps",
			uploadFunc: func() error {
				return client.UploadFileToPath(sourcePath, "/tmp/file.db")
			},
		},
		{
			name: "upload file remote name traversal",
			uploadFunc: func() error {
				return client.UploadFile(sourcePath, "../other-app/file.db")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			networkCalled = false
			err := tc.uploadFunc()
			if err == nil || !strings.Contains(err.Error(), "Baidu app directory") {
				t.Fatalf("expected Baidu app directory escape error, got %v", err)
			}
			if networkCalled {
				t.Fatal("Baidu API should not be called for escaped remote path")
			}
		})
	}
}

func TestBaiduPCSClientRejectsRemotePathEscapesAcrossOperations(t *testing.T) {
	networkCalled := false
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	localTarget := filepath.Join(t.TempDir(), "download.db")
	escapedFilePath := path.Join("/apps", syncApp, "..", "other-app", "file.db")
	escapedDirPath := path.Join("/apps", syncApp+"-evil")

	cases := []struct {
		name      string
		operation func() error
	}{
		{name: "ensure directories", operation: func() error { return client.EnsureRemoteDirectories(escapedDirPath) }},
		{name: "create directory", operation: func() error { return client.CreateDirectory(escapedDirPath) }},
		{name: "list files", operation: func() error { _, err := client.ListFiles(escapedDirPath); return err }},
		{name: "list recursive", operation: func() error { _, err := client.ListFilesRecursive(escapedDirPath); return err }},
		{name: "get file info", operation: func() error { _, err := client.GetFileInfo(escapedFilePath); return err }},
		{name: "download file", operation: func() error { return client.DownloadFile(escapedFilePath, localTarget) }},
		{name: "delete file", operation: func() error { return client.DeleteFile(escapedFilePath) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			networkCalled = false
			err := tc.operation()
			if err == nil || !strings.Contains(err.Error(), "Baidu app directory") {
				t.Fatalf("expected Baidu app directory escape error, got %v", err)
			}
			if networkCalled {
				t.Fatal("Baidu API should not be called for escaped remote path")
			}
		})
	}
}

func TestBaiduPCSClientRejectsDeletingAppRootBeforeRequest(t *testing.T) {
	networkCalled := false
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	err := client.DeleteFile(path.Join("/apps", syncApp))
	if err == nil || !strings.Contains(err.Error(), "app root") {
		t.Fatalf("expected app root deletion error, got %v", err)
	}
	if networkCalled {
		t.Fatal("Baidu API should not be called when deleting app root")
	}
}

func TestBaiduPCSClientListFilesSkipsUnexpectedRemotePaths(t *testing.T) {
	root := syncRemoteRootPath()
	validDir := path.Join(root, "papers")
	validFile := path.Join(root, "data", "diveend.db")
	validFileMtime := int64(1710000000)
	listedDirs := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			dir := r.URL.Query().Get("dir")
			listedDirs = append(listedDirs, dir)
			switch dir {
			case root:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errno": 0,
					"list": []map[string]any{
						{"path": validDir, "isdir": "1", "size": "0"},
						{"path": path.Join("/apps", syncApp+"-evil"), "isdir": 1, "size": 0},
						{"path": root + "/papers/../escaped", "isdir": 1, "size": 0},
						{"path": path.Join("/apps", syncApp, "..", "other-app", "file.db"), "isdir": 0, "size": 1},
						{"path": path.Join("/apps", syncApp), "isdir": 0, "size": 1},
					},
				})
			case validDir:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errno": 0,
					"list": []map[string]any{
						{"path": validFile, "isdir": "0", "size": "2", "server_mtime": fmt.Sprint(validFileMtime)},
						{"path": path.Join("/apps", syncApp+"-evil", "nested.db"), "isdir": 0, "size": 1},
					},
				})
			default:
				t.Fatalf("unexpected list dir %q", dir)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	files, err := client.ListFiles(root)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != validDir || !files[0].IsDir {
		t.Fatalf("expected only valid directory entry, got %+v", files)
	}

	recursiveFiles, err := client.ListFilesRecursive(root)
	if err != nil {
		t.Fatalf("ListFilesRecursive() error = %v", err)
	}
	if len(recursiveFiles) != 1 || recursiveFiles[0].Path != validFile || recursiveFiles[0].IsDir {
		t.Fatalf("expected only valid recursive file, got %+v", recursiveFiles)
	}
	if recursiveFiles[0].Size != 2 || recursiveFiles[0].Modified.Unix() != validFileMtime {
		t.Fatalf("expected parsed size and mtime from list response, got %+v", recursiveFiles[0])
	}
	if !stringSlicesEqual(listedDirs, []string{root, root, validDir}) {
		t.Fatalf("unexpected list dirs: %+v", listedDirs)
	}
}

func TestOpenBaiduUploadSourceRejectsFileSwapDuringOpen(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.db")
	if err := os.WriteFile(sourcePath, []byte("original"), 0600); err != nil {
		t.Fatalf("WriteFile original source error = %v", err)
	}

	swapped := false
	file, _, err := openBaiduUploadSourceWithOpen(sourcePath, func(openPath string) (*os.File, error) {
		if !swapped {
			swapped = true
			if err := os.Remove(openPath); err != nil {
				t.Fatalf("Remove original source error = %v", err)
			}
			if err := os.WriteFile(openPath, []byte("replacement"), 0600); err != nil {
				t.Fatalf("WriteFile replacement source error = %v", err)
			}
		}
		return os.Open(openPath)
	})
	if file != nil {
		_ = file.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("expected file swap rejection, got %v", err)
	}
}

func TestBaiduPCSClientUploadRejectsChunkMD5Mismatch(t *testing.T) {
	var uploadRequested bool
	var createRequested bool
	uploadedPath := path.Join(syncRemoteRootPath(), "data", "mismatch.db")
	uploadID := "mismatch-upload"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			switch r.URL.Query().Get("method") {
			case "create":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm create error = %v", err)
				}
				if r.Form.Get("isdir") == "1" {
					_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": 1, "path": r.Form.Get("path")})
					return
				}
				createRequested = true
				http.Error(w, "create should not be called", http.StatusInternalServerError)
			case "precreate":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm precreate error = %v", err)
				}
				if got := r.Form.Get("path"); got != uploadedPath {
					t.Fatalf("unexpected precreate path %q", got)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "uploadid": uploadID})
			default:
				http.NotFound(w, r)
			}
		case "/rest/2.0/pcs/superfile2":
			uploadRequested = true
			_ = json.NewEncoder(w).Encode(map[string]any{"md5": "definitely-not-the-local-md5"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	sourcePath := filepath.Join(t.TempDir(), "mismatch.db")
	if err := os.WriteFile(sourcePath, []byte("content"), 0600); err != nil {
		t.Fatalf("WriteFile source error = %v", err)
	}

	err = client.UploadFileToPath(sourcePath, uploadedPath)
	if err == nil || !strings.Contains(err.Error(), "md5 mismatch") {
		t.Fatalf("expected md5 mismatch error, got %v", err)
	}
	if !uploadRequested {
		t.Fatal("expected chunk upload to be attempted")
	}
	if createRequested {
		t.Fatal("create should not be called after chunk md5 mismatch")
	}
}

func TestBaiduPCSDefaultHTTPClientsUseTimeout(t *testing.T) {
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "token"})
	if client.httpClient == nil {
		t.Fatal("expected Baidu client to initialize an HTTP client")
	}
	if client.httpClient.Timeout != baiduHTTPTimeout {
		t.Fatalf("expected Baidu client timeout %v, got %v", baiduHTTPTimeout, client.httpClient.Timeout)
	}

	refreshClient := defaultBaiduHTTPClient()
	if refreshClient.Timeout != baiduHTTPTimeout {
		t.Fatalf("expected refresh HTTP client timeout %v, got %v", baiduHTTPTimeout, refreshClient.Timeout)
	}
}

func TestBaiduPCSClientLimitsJSONResponses(t *testing.T) {
	previousLimit := externalHTTPBodyLimitBytes
	externalHTTPBodyLimitBytes = 16
	t.Cleanup(func() { externalHTTPBodyLimitBytes = previousLimit })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_, _ = w.Write([]byte(`{"errno":0,"padding":"` + strings.Repeat("x", 17) + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	_, err = client.GetAccessToken()
	if err == nil {
		t.Fatal("expected GetAccessToken() to reject oversized JSON response")
	}
	if !strings.Contains(err.Error(), "response body exceeds 16 byte limit") {
		t.Fatalf("expected response body limit error, got %v", err)
	}
}

func TestBaiduPCSClientDownloadFailurePreservesExistingFile(t *testing.T) {
	const remotePath = "/apps/pcstest_oauth/diveend-v1/papers/broken.pdf"
	const existingContent = "existing complete file"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && token != "valid-token" {
			t.Fatalf("unexpected access token %q", token)
		}

		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"path": remotePath, "fs_id": 1001, "server_filename": "broken.pdf", "size": 100},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 1001, "path": remotePath, "dlink": "https://d.pcs.baidu.com/download/broken.pdf"},
				},
			})
		case "/download/broken.pdf":
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("partial"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	downloadTarget := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(downloadTarget, []byte(existingContent), 0600); err != nil {
		t.Fatalf("WriteFile existing target error = %v", err)
	}

	err = client.DownloadFile(remotePath, downloadTarget)
	if err == nil {
		t.Fatal("expected DownloadFile() to fail on truncated response")
	}
	downloaded, readErr := os.ReadFile(downloadTarget)
	if readErr != nil {
		t.Fatalf("ReadFile target after failed download error = %v", readErr)
	}
	if string(downloaded) != existingContent {
		t.Fatalf("expected existing file to be preserved, got %q", string(downloaded))
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(downloadTarget), "."+filepath.Base(downloadTarget)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp download files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected failed download temp files to be cleaned, got %v", tempMatches)
	}
}

func TestBaiduPCSClientDownloadRejectsSymlinkedOutputDirectoryBeforeRequest(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	if err := os.Symlink(outsideDir, outputDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	networkCalled := false
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		networkCalled = true
		return nil, fmt.Errorf("network should not be called")
	})}

	err := client.DownloadFile("/apps/pcstest_oauth/diveend-v1/papers/remote.pdf", filepath.Join(outputDir, "remote.pdf"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlinked output directory error, got %v", err)
	}
	if networkCalled {
		t.Fatal("Baidu API should not be called when output directory is a symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "remote.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no download to be written through symlinked output directory, stat err=%v", statErr)
	}
}

func TestBaiduPCSClientDownloadSilentTruncationPreservesExistingFile(t *testing.T) {
	const remotePath = "/apps/pcstest_oauth/diveend-v1/papers/silent-truncate.pdf"
	const existingContent = "existing complete file"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"path": remotePath, "fs_id": 1002, "server_filename": "silent-truncate.pdf", "size": 100},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 1002, "path": remotePath, "dlink": "https://d.pcs.baidu.com/download/silent-truncate.pdf"},
				},
			})
		case "/download/silent-truncate.pdf":
			_, _ = w.Write([]byte("partial"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	downloadTarget := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(downloadTarget, []byte(existingContent), 0600); err != nil {
		t.Fatalf("WriteFile existing target error = %v", err)
	}

	err = client.DownloadFile(remotePath, downloadTarget)
	if err == nil {
		t.Fatal("expected DownloadFile() to fail on silent truncated response")
	}
	if !strings.Contains(err.Error(), "downloaded size mismatch") {
		t.Fatalf("expected size mismatch error, got %v", err)
	}
	downloaded, readErr := os.ReadFile(downloadTarget)
	if readErr != nil {
		t.Fatalf("ReadFile target after failed download error = %v", readErr)
	}
	if string(downloaded) != existingContent {
		t.Fatalf("expected existing file to be preserved, got %q", string(downloaded))
	}
}

func TestBaiduPCSClientRejectsOversizedRemoteDownload(t *testing.T) {
	previousLimit := baiduDownloadMaxBytes
	baiduDownloadMaxBytes = 8
	t.Cleanup(func() { baiduDownloadMaxBytes = previousLimit })

	const remotePath = "/apps/pcstest_oauth/diveend-v1/papers/huge.pdf"
	downloadRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"path": remotePath, "fs_id": 1003, "server_filename": "huge.pdf", "size": 9},
				},
			})
		case "/rest/2.0/xpan/multimedia", "/download/huge.pdf":
			downloadRequested = true
			http.Error(w, "should not be requested", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	downloadTarget := filepath.Join(t.TempDir(), "huge.pdf")
	err = client.DownloadFile(remotePath, downloadTarget)
	if err == nil {
		t.Fatal("expected DownloadFile() to reject oversized remote file")
	}
	if !strings.Contains(err.Error(), "remote file exceeds download limit") {
		t.Fatalf("expected download limit error, got %v", err)
	}
	if downloadRequested {
		t.Fatal("oversized remote file should be rejected before requesting dlink or body")
	}
	if _, statErr := os.Stat(downloadTarget); !os.IsNotExist(statErr) {
		t.Fatalf("expected no target file to be created, stat err = %v", statErr)
	}
}

func TestBaiduDownloadURLWithAccessTokenPreservesExistingQuery(t *testing.T) {
	downloadURL, err := baiduDownloadURLWithAccessToken("https://d.pcs.baidu.com/download/file.pdf?fid=abc&access_token=old", "new-token")
	if err != nil {
		t.Fatalf("baiduDownloadURLWithAccessToken() error = %v", err)
	}
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		t.Fatalf("Parse(downloadURL) error = %v", err)
	}
	if parsed.Query().Get("fid") != "abc" {
		t.Fatalf("expected existing query parameter to be preserved, got %q", parsed.RawQuery)
	}
	if parsed.Query().Get("access_token") != "new-token" {
		t.Fatalf("expected access_token to be replaced, got %q", parsed.RawQuery)
	}
	if strings.Contains(downloadURL, "?fid=abc?access_token=") {
		t.Fatalf("download URL was built with duplicate question marks: %s", downloadURL)
	}
}

func TestBaiduPCSClientRejectsUnexpectedDownloadLinkHost(t *testing.T) {
	const remotePath = "/apps/pcstest_oauth/diveend-v1/papers/untrusted-host.pdf"
	const existingContent = "existing complete file"
	downloadRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"path": remotePath, "fs_id": 1004, "server_filename": "untrusted-host.pdf", "size": len(existingContent)},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 1004, "path": remotePath, "dlink": "https://evil.example/download/untrusted-host.pdf?fid=abc"},
				},
			})
		case "/download/untrusted-host.pdf":
			downloadRequested = true
			http.Error(w, "should not download from unexpected host", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	downloadTarget := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(downloadTarget, []byte(existingContent), 0600); err != nil {
		t.Fatalf("WriteFile existing target error = %v", err)
	}

	err = client.DownloadFile(remotePath, downloadTarget)
	if err == nil || !strings.Contains(err.Error(), "not a Baidu host") {
		t.Fatalf("expected unexpected host error, got %v", err)
	}
	if downloadRequested {
		t.Fatal("unexpected download host should be rejected before requesting the body")
	}
	downloaded, readErr := os.ReadFile(downloadTarget)
	if readErr != nil {
		t.Fatalf("ReadFile target after failed download error = %v", readErr)
	}
	if string(downloaded) != existingContent {
		t.Fatalf("expected existing file to be preserved, got %q", string(downloaded))
	}
}

func TestBaiduPCSClientRejectsDownloadRedirectToPrivateHost(t *testing.T) {
	const remotePath = "/apps/pcstest_oauth/diveend-v1/papers/private-redirect.pdf"
	const existingContent = "existing complete file"
	privateDownloadRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/rest/2.0/xpan/file":
			if r.URL.Query().Get("method") != "list" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"path": remotePath, "fs_id": 1005, "server_filename": "private-redirect.pdf", "size": len(existingContent)},
				},
			})
		case "/rest/2.0/xpan/multimedia":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errno": 0,
				"list": []map[string]any{
					{"fs_id": 1005, "path": remotePath, "dlink": "https://d.pcs.baidu.com/download/private-redirect.pdf"},
				},
			})
		case "/download/private-redirect.pdf":
			http.Redirect(w, r, "http://127.0.0.1/private.pdf", http.StatusFound)
		case "/private.pdf":
			privateDownloadRequested = true
			_, _ = w.Write([]byte(existingContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}
	client := NewBaiduPCSClient(&BaiduToken{AccessToken: "valid-token"})
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	downloadTarget := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(downloadTarget, []byte(existingContent), 0600); err != nil {
		t.Fatalf("WriteFile existing target error = %v", err)
	}

	err = client.DownloadFile(remotePath, downloadTarget)
	if err == nil || !strings.Contains(err.Error(), "host is not allowed") {
		t.Fatalf("expected private redirect host error, got %v", err)
	}
	if privateDownloadRequested {
		t.Fatal("private redirect target should not be requested")
	}
	downloaded, readErr := os.ReadFile(downloadTarget)
	if readErr != nil {
		t.Fatalf("ReadFile target after failed download error = %v", readErr)
	}
	if string(downloaded) != existingContent {
		t.Fatalf("expected existing file to be preserved, got %q", string(downloaded))
	}
}

func TestBaiduPCSClientRefreshesExpiredTokenAndPersistsSecurely(t *testing.T) {
	var refreshCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 1})
		case "/oauth/2.0/token":
			refreshCalled = true
			query := r.URL.Query()
			if query.Get("grant_type") != "refresh_token" {
				t.Fatalf("unexpected grant_type %q", query.Get("grant_type"))
			}
			if query.Get("refresh_token") != "old-refresh" || query.Get("client_id") != "client-id" || query.Get("client_secret") != "client-secret" {
				t.Fatalf("unexpected refresh query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "new-access",
				// Some Baidu responses omit refresh_token. The client must retain
				// the existing refresh token instead of erasing it.
				"expires_in": 3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	tokenPath := filepath.Join(t.TempDir(), "nested", "baiduyun_token.json")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		t.Fatalf("MkdirAll token dir error = %v", err)
	}
	if err := os.WriteFile(tokenPath, []byte(`{"accessToken":"stale"}`), 0644); err != nil {
		t.Fatalf("WriteFile existing token error = %v", err)
	}
	client := NewBaiduPCSClientWithTokenPath(&BaiduToken{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, tokenPath)
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	accessToken, err := client.GetAccessToken()
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if accessToken != "new-access" {
		t.Fatalf("expected refreshed access token, got %q", accessToken)
	}
	if !refreshCalled {
		t.Fatal("expected refresh endpoint to be called")
	}

	saved, err := LoadBaiduToken(tokenPath)
	if err != nil {
		t.Fatalf("LoadBaiduToken() error = %v", err)
	}
	if saved.AccessToken != "new-access" || saved.RefreshToken != "old-refresh" || saved.ClientID != "client-id" || saved.ClientSecret != "client-secret" {
		t.Fatalf("unexpected saved token: %+v", saved)
	}
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Stat token file error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected token file permission 0600, got %o", info.Mode().Perm())
	}
	tempMatches, err := filepath.Glob(filepath.Join(filepath.Dir(tokenPath), ".baiduyun_token.json.tmp-*"))
	if err != nil {
		t.Fatalf("Glob temp token files error = %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no leftover temp token files, got %v", tempMatches)
	}
}

func TestBaiduPCSClientSerializesConcurrentTokenRefresh(t *testing.T) {
	var refreshRequests int32
	var quotaRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/quota":
			atomic.AddInt32(&quotaRequests, 1)
			if r.URL.Query().Get("access_token") == "old-access" {
				_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 1})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "expire": 0})
		case "/oauth/2.0/token":
			atomic.AddInt32(&refreshRequests, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL error = %v", err)
	}

	tokenPath := filepath.Join(t.TempDir(), "baiduyun_token.json")
	client := NewBaiduPCSClientWithTokenPath(&BaiduToken{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, tokenPath)
	client.httpClient = &http.Client{Transport: baiduRewriteTransport{target: targetURL}}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			accessToken, err := client.GetAccessToken()
			if err != nil {
				errs <- err
				return
			}
			if accessToken != "new-access" {
				errs <- fmt.Errorf("unexpected access token %q", accessToken)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := atomic.LoadInt32(&refreshRequests); got != 1 {
		t.Fatalf("expected one serialized refresh request, got %d", got)
	}
	if got := atomic.LoadInt32(&quotaRequests); got < workers {
		t.Fatalf("expected at least %d quota checks, got %d", workers, got)
	}

	saved, err := LoadBaiduToken(tokenPath)
	if err != nil {
		t.Fatalf("LoadBaiduToken() error = %v", err)
	}
	if saved.AccessToken != "new-access" || saved.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected saved token: %+v", saved)
	}
}

func handleBaiduFileContract(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
	uploadedPath string,
	uploadID string,
	createdDirs *[]string,
	uploadChunks map[int][]byte,
	remoteObjects map[string]baiduRemoteObject,
	fsIDToPath map[string]string,
	nextID func() string,
) {
	t.Helper()

	switch r.URL.Query().Get("method") {
	case "create":
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm create error = %v", err)
		}
		if r.Form.Get("isdir") == "1" {
			*createdDirs = append(*createdDirs, r.Form.Get("path"))
			_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": nextID(), "path": r.Form.Get("path")})
			return
		}

		if got := r.Form.Get("path"); got != uploadedPath {
			t.Fatalf("unexpected create path %q", got)
		}
		if got := r.Form.Get("uploadid"); got != uploadID {
			t.Fatalf("unexpected uploadid %q", got)
		}
		blockList := decodeBlockList(t, r.Form.Get("block_list"))
		content := joinChunks(uploadChunks, len(blockList))
		if got, want := fmt.Sprint(len(content)), r.Form.Get("size"); got != want {
			t.Fatalf("unexpected create size got %s want %s", got, want)
		}
		if !stringSlicesEqual(blockMD5s(content), blockList) {
			t.Fatalf("create block_list does not match uploaded content")
		}
		fsID := nextID()
		remoteObjects[uploadedPath] = baiduRemoteObject{
			content: append([]byte(nil), content...),
			fsID:    fsID,
			md5:     md5Hex(content),
		}
		fsIDToPath[fsID] = uploadedPath
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "fs_id": fsID, "path": uploadedPath})
	case "precreate":
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm precreate error = %v", err)
		}
		if got := r.Form.Get("path"); got != uploadedPath {
			t.Fatalf("unexpected precreate path %q", got)
		}
		if len(decodeBlockList(t, r.Form.Get("block_list"))) != 2 {
			t.Fatalf("expected precreate block_list to contain 2 blocks")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "uploadid": uploadID})
	case "list":
		dir := r.URL.Query().Get("dir")
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": listRemoteObjects(dir, remoteObjects)})
	case "filemanager":
		if r.URL.Query().Get("opera") != "delete" {
			t.Fatalf("unexpected filemanager opera %q", r.URL.Query().Get("opera"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm delete error = %v", err)
		}
		if got := decodeBlockList(t, r.Form.Get("filelist")); len(got) == 0 {
			t.Fatalf("expected delete filelist to contain at least one path")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0})
	default:
		t.Fatalf("unexpected xpan file method %q", r.URL.Query().Get("method"))
	}
}

func handleBaiduUploadContract(t *testing.T, w http.ResponseWriter, r *http.Request, uploadedPath, uploadID string, uploadChunks map[int][]byte) {
	t.Helper()
	if r.URL.Query().Get("method") != "upload" {
		t.Fatalf("unexpected upload method %q", r.URL.Query().Get("method"))
	}
	if got := r.URL.Query().Get("path"); got != uploadedPath {
		t.Fatalf("unexpected upload path %q", got)
	}
	if got := r.URL.Query().Get("uploadid"); got != uploadID {
		t.Fatalf("unexpected uploadid %q", got)
	}
	partSeq, err := strconv.Atoi(r.URL.Query().Get("partseq"))
	if err != nil {
		t.Fatalf("bad partseq: %v", err)
	}
	reader, err := r.MultipartReader()
	if err != nil {
		t.Fatalf("MultipartReader error = %v", err)
	}
	var payload []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart error = %v", err)
		}
		if part.FormName() != "file" {
			continue
		}
		payload, err = io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll part error = %v", err)
		}
	}
	if len(payload) == 0 {
		t.Fatal("expected upload chunk payload")
	}
	uploadChunks[partSeq] = append([]byte(nil), payload...)
	_ = json.NewEncoder(w).Encode(map[string]any{"md5": md5Hex(payload)})
}

func handleBaiduMetaContract(t *testing.T, w http.ResponseWriter, r *http.Request, fsIDToPath map[string]string) {
	t.Helper()
	var fsIDs []json.Number
	if err := json.Unmarshal([]byte(r.URL.Query().Get("fsids")), &fsIDs); err != nil {
		t.Fatalf("failed to parse fsids: %v", err)
	}
	entries := make([]map[string]any, 0, len(fsIDs))
	for _, fsID := range fsIDs {
		fsIDString := fsID.String()
		if _, ok := fsIDToPath[fsIDString]; !ok {
			continue
		}
		entries = append(entries, map[string]any{"fs_id": fsIDString, "dlink": "https://d.pcs.baidu.com/download/" + fsIDString})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"errno": 0, "list": entries})
}

func decodeBlockList(t *testing.T, raw string) []string {
	t.Helper()
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("decode block list %q error = %v", raw, err)
	}
	return values
}

func joinChunks(chunks map[int][]byte, count int) []byte {
	var joined []byte
	for i := 0; i < count; i++ {
		joined = append(joined, chunks[i]...)
	}
	return joined
}

func blockMD5s(content []byte) []string {
	result := []string{}
	for len(content) > 0 {
		n := blockSize
		if len(content) < n {
			n = len(content)
		}
		result = append(result, md5Hex(content[:n]))
		content = content[n:]
	}
	return result
}

func md5Hex(content []byte) string {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:])
}

func listRemoteObjects(dir string, objects map[string]baiduRemoteObject) []map[string]any {
	dir = path.Clean(dir)
	seenDirs := map[string]bool{}
	entries := []map[string]any{}
	for objectPath, object := range objects {
		parent := path.Dir(objectPath)
		if parent == dir {
			entries = append(entries, map[string]any{
				"path":         objectPath,
				"size":         len(object.content),
				"isdir":        0,
				"fs_id":        object.fsID,
				"md5":          object.md5,
				"server_mtime": timeUnixForTest(),
			})
			continue
		}
		if strings.HasPrefix(objectPath, strings.TrimRight(dir, "/")+"/") {
			rest := strings.TrimPrefix(objectPath, strings.TrimRight(dir, "/")+"/")
			next := strings.Split(rest, "/")[0]
			childDir := path.Join(dir, next)
			if !seenDirs[childDir] {
				seenDirs[childDir] = true
				entries = append(entries, map[string]any{
					"path":         childDir,
					"size":         0,
					"isdir":        1,
					"server_mtime": timeUnixForTest(),
				})
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return fmt.Sprint(entries[i]["path"]) < fmt.Sprint(entries[j]["path"])
	})
	return entries
}

func timeUnixForTest() int64 {
	return 1781148000
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
