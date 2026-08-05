package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestGoogleDriveRelativeParts(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{name: "root", path: syncRemoteRootPath(), want: nil},
		{name: "manifest", path: syncRemotePathForKey(syncManifestFileName), want: []string{syncManifestFileName}},
		{name: "nested", path: syncRemotePathForKey("papers/example.pdf"), want: []string{"papers", "example.pdf"}},
		{name: "outside root", path: "/apps/other/file.json", wantErr: true},
		{name: "parent escape", path: syncRemoteRootPath() + "/../outside.json", wantErr: true},
		{name: "empty segment", path: syncRemoteRootPath() + "/papers//file.pdf", want: []string{"papers", "file.pdf"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := googleDriveRelativeParts(test.path)
			if (err != nil) != test.wantErr {
				t.Fatalf("googleDriveRelativeParts(%q) error = %v, wantErr %v", test.path, err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if len(got) != len(test.want) {
				t.Fatalf("googleDriveRelativeParts(%q) = %#v, want %#v", test.path, got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("googleDriveRelativeParts(%q) = %#v, want %#v", test.path, got, test.want)
				}
			}
		})
	}
}

func TestGoogleDriveTokenRoundTripUsesPrivateFile(t *testing.T) {
	useTestConfigPath(t)
	tokenPath := defaultGoogleDriveTokenPath()
	want := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
	}

	if err := saveGoogleDriveToken(tokenPath, want); err != nil {
		t.Fatalf("saveGoogleDriveToken() error = %v", err)
	}
	got, err := loadGoogleDriveToken(tokenPath)
	if err != nil {
		t.Fatalf("loadGoogleDriveToken() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("loaded token = %#v, want %#v", got, want)
	}
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("stat token file error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("token file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestGoogleDriveReadyForSyncRequiresClientAndRefreshToken(t *testing.T) {
	useTestConfigPath(t)
	config := GoogleDriveConfig{
		Enabled:          true,
		ClientSecretPath: filepath.Join("config", "google_drive_client.json"),
		RootFolderName:   "DiveEnd Backup",
	}
	if googleDriveReadyForSync(config) {
		t.Fatal("googleDriveReadyForSync() = true without client JSON and token")
	}

	clientPath := filepath.Join("config", "google_drive_client.json")
	if err := os.MkdirAll(filepath.Dir(clientPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	clientJSON := `{"installed":{"client_id":"desktop-client","client_secret":"desktop-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(clientPath, []byte(clientJSON), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := saveGoogleDriveToken(defaultGoogleDriveTokenPath(), &oauth2.Token{RefreshToken: "refresh-token"}); err != nil {
		t.Fatalf("saveGoogleDriveToken() error = %v", err)
	}
	if !googleDriveReadyForSync(config) {
		t.Fatal("googleDriveReadyForSync() = false with valid client JSON and refresh token")
	}
}

func TestPrepareProviderForRunFallsBackBeforeUploadOnly(t *testing.T) {
	google := &testSyncProvider{name: "google_drive", checkErr: errors.New("offline")}
	baidu := &testSyncProvider{name: "baidu_cloud"}
	manager := &SyncManager{
		primary:  google,
		provider: google,
		fallback: baidu,
		config:   AppConfig{GoogleDrive: GoogleDriveConfig{FallbackToBaidu: true}},
	}

	provider, err := manager.prepareProviderForRun()
	if err != nil {
		t.Fatalf("prepareProviderForRun() error = %v", err)
	}
	if provider != baidu || manager.activeProvider() != baidu {
		t.Fatalf("expected Baidu fallback provider, got provider=%v active=%v", provider, manager.activeProvider())
	}
	manager.restorePrimaryProvider()
	if manager.activeProvider() != google {
		t.Fatalf("restorePrimaryProvider() active provider = %v, want Google", manager.activeProvider())
	}
}

func TestPrepareProviderForRunDoesNotFallbackWhenDisabled(t *testing.T) {
	google := &testSyncProvider{name: "google_drive", checkErr: errors.New("offline")}
	baidu := &testSyncProvider{name: "baidu_cloud"}
	manager := &SyncManager{
		primary:  google,
		provider: google,
		fallback: baidu,
		config:   AppConfig{GoogleDrive: GoogleDriveConfig{FallbackToBaidu: false}},
	}

	if _, err := manager.prepareProviderForRun(); err == nil {
		t.Fatal("prepareProviderForRun() error = nil, want Google preflight failure")
	}
	if manager.activeProvider() != google {
		t.Fatalf("active provider changed after disabled fallback: %v", manager.activeProvider())
	}
}

func TestActiveProviderDoesNotBypassDisabledFallback(t *testing.T) {
	baidu := &BaiduPCSClient{}
	manager := &SyncManager{baiduClient: baidu, fallback: baidu}
	if manager.activeProvider() != nil {
		t.Fatal("activeProvider() returned Baidu even though Google fallback is explicitly unavailable")
	}

	legacyManager := &SyncManager{baiduClient: baidu}
	if legacyManager.activeProvider() != baidu {
		t.Fatal("legacy manager did not retain Baidu provider compatibility")
	}
}

func TestSyncConfiguredRespectsGoogleFallbackSetting(t *testing.T) {
	useTestConfigPath(t)
	config := AppConfig{
		GoogleDrive: GoogleDriveConfig{Enabled: true, FallbackToBaidu: false},
		BaiduCloud:  BaiduCloudConfig{Enabled: true, Token: "baidu-token"},
	}
	if syncConfigured(config) {
		t.Fatal("syncConfigured() = true when Google is enabled but unauthorized and fallback is disabled")
	}

	config.GoogleDrive.Enabled = false
	if !syncConfigured(config) {
		t.Fatal("syncConfigured() = false for a configured Baidu-only setup")
	}
}

type testSyncProvider struct {
	name     string
	checkErr error
}

func (p *testSyncProvider) Name() string { return p.name }

func (p *testSyncProvider) Check() error { return p.checkErr }

func (p *testSyncProvider) UploadFileToPath(string, string) error { return nil }

func (p *testSyncProvider) DownloadFile(string, string) error { return nil }

func (p *testSyncProvider) ListFilesRecursive(string) ([]FileInfo, error) { return nil, nil }

func (p *testSyncProvider) EnsureRemoteDirectories(string) error { return nil }

func (p *testSyncProvider) RemoteRootLabel() string { return filepath.Join(p.name, "DiveEnd") }
