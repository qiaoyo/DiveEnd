package app

import (
	"fmt"
	"log"
)

// syncProvider is the small cloud contract used by the provider-neutral
// snapshot, manifest, conflict, and restore workflow.
type syncProvider interface {
	Name() string
	Check() error
	UploadFileToPath(localPath, remotePath string) error
	DownloadFile(remotePath, localPath string) error
	ListFilesRecursive(remoteRoot string) ([]FileInfo, error)
	EnsureRemoteDirectories(remoteDir string) error
	RemoteRootLabel() string
}

func (sm *SyncManager) activeProvider() syncProvider {
	if sm == nil {
		return nil
	}
	if sm.provider != nil {
		return sm.provider
	}
	// Keep tests and older callers that construct a manager with only the
	// legacy field working while preventing a configured-but-disabled fallback
	// from silently becoming the active provider.
	if sm.primary == nil && sm.fallback == nil {
		return sm.baiduClient
	}
	return nil
}

func (sm *SyncManager) providerOrError() (syncProvider, error) {
	provider := sm.activeProvider()
	if provider == nil {
		return nil, fmt.Errorf("no cloud sync provider is configured")
	}
	return provider, nil
}

// prepareProviderForRun performs one preflight before any snapshot is
// uploaded. Google Drive is primary; Baidu is used only when Google is
// unavailable before the run starts, which avoids mixing two providers after
// a partial upload.
func (sm *SyncManager) prepareProviderForRun() (syncProvider, error) {
	provider := sm.primary
	if provider == nil {
		provider = sm.activeProvider()
	}
	if provider == nil {
		return nil, fmt.Errorf("no cloud sync provider is configured")
	}
	err := provider.Check()
	if err == nil {
		return provider, nil
	} else if provider.Name() != "google_drive" || !sm.config.GoogleDrive.FallbackToBaidu || sm.fallback == nil {
		return nil, fmt.Errorf("%s provider preflight failed: %w", provider.Name(), err)
	} else {
		fallbackErr := sm.fallback.Check()
		if fallbackErr != nil {
			return nil, fmt.Errorf("Google Drive preflight failed: %v; Baidu fallback preflight failed: %w", err, fallbackErr)
		}
		sm.provider = sm.fallback
		log.Printf("[Sync] Google Drive unavailable; using Baidu Cloud fallback for this run")
		return sm.fallback, nil
	}
}

func (sm *SyncManager) restorePrimaryProvider() {
	if sm != nil && sm.primary != nil {
		sm.provider = sm.primary
	}
}

func syncProviderName(provider syncProvider) string {
	if provider == nil {
		return "none"
	}
	return provider.Name()
}

func syncProviderCredentialPath(provider syncProvider) string {
	switch provider.(type) {
	case *GoogleDriveProvider:
		return defaultGoogleDriveTokenPath()
	case *BaiduPCSClient:
		return defaultBaiduTokenPath()
	default:
		return ""
	}
}

func (c *BaiduPCSClient) Name() string {
	return "baidu_cloud"
}

func (c *BaiduPCSClient) Check() error {
	_, err := c.GetAccessToken()
	return err
}

func (c *BaiduPCSClient) RemoteRootLabel() string {
	return syncRemoteRootPath()
}
