package app

import "github.com/qiaoyo/DiveEnd/internal/domain"

// Keep the Wails-facing aliases in package main. The concrete API contracts
// live in internal/domain so non-Wails packages can depend on them without
// importing the application entrypoint.
const defaultFolderName = domain.DefaultFolderName

type LLMConfig = domain.LLMConfig
type SearchAPIConfig = domain.SearchAPIConfig
type BaiduCloudConfig = domain.BaiduCloudConfig
type AppConfig = domain.AppConfig
type SaveConfigResult = domain.SaveConfigResult
type ConfigSecretPrefill = domain.ConfigSecretPrefill
type InitialState = domain.InitialState
type DeepStartAIRequest = domain.DeepStartAIRequest
type DeepStartAIResponse = domain.DeepStartAIResponse
type ScreeningAIRequest = domain.ScreeningAIRequest
type ScreeningPaper = domain.ScreeningPaper
type ScreeningSession = domain.ScreeningSession
type ScreeningDecisionOption = domain.ScreeningDecisionOption
type ScreeningDecisionNode = domain.ScreeningDecisionNode
type ExtractProgress = domain.ExtractProgress
type ScreeningSessionDetail = domain.ScreeningSessionDetail
type PathHistoryItem = domain.PathHistoryItem
type Folder = domain.Folder
type FolderNode = domain.FolderNode
type CreateFolderNodeRequest = domain.CreateFolderNodeRequest
type RenameFolderNodeRequest = domain.RenameFolderNodeRequest
type MoveFolderNodeRequest = domain.MoveFolderNodeRequest
type Paper = domain.Paper
type SearchPaper = domain.SearchPaper
type PaperProfileExtraction = domain.PaperProfileExtraction
type ImportSkippedPaper = domain.ImportSkippedPaper
type ImportPapersWithAssetsResult = domain.ImportPapersWithAssetsResult
type LocalStorageFolderOverview = domain.LocalStorageFolderOverview
type FolderStorageTreeNode = domain.FolderStorageTreeNode
type FolderStorageTreeOverview = domain.FolderStorageTreeOverview
type LocalStorageOverview = domain.LocalStorageOverview
type SearchRetrievalStats = domain.SearchRetrievalStats
type SearchSourceStatus = domain.SearchSourceStatus
type EnhancedSearchResult = domain.EnhancedSearchResult
type DeepStartSessionSummary = domain.DeepStartSessionSummary
type DeepStartMessage = domain.DeepStartMessage
type DeepStartDirection = domain.DeepStartDirection
type DeepStartPaperNote = domain.DeepStartPaperNote
type DeepStartAnalysis = domain.DeepStartAnalysis
type DeepStartSessionDetail = domain.DeepStartSessionDetail
type DeepStartProgressEvent = domain.DeepStartProgressEvent
type DeepStartEnrichmentCache = domain.DeepStartEnrichmentCache
type TranslationRecord = domain.TranslationRecord
type DeepReadSection = domain.DeepReadSection
type DeepReadParseCache = domain.DeepReadParseCache
type DeepReadNote = domain.DeepReadNote
type DeepReadEvidence = domain.DeepReadEvidence
type DeepReadAIResponse = domain.DeepReadAIResponse
type DeepReadState = domain.DeepReadState
type BaiduToken = domain.BaiduToken
type SyncRecord = domain.SyncRecord
type SyncConflict = domain.SyncConflict
type SyncStatus = domain.SyncStatus
type SyncSettings = domain.SyncSettings
type SyncProgress = domain.SyncProgress
type BaiduTokenRefreshStatus = domain.BaiduTokenRefreshStatus
type SyncPreviewFile = domain.SyncPreviewFile
type SyncPreview = domain.SyncPreview
type PDFServiceStatus = domain.PDFServiceStatus
type DatabaseRestoreStatus = domain.DatabaseRestoreStatus
type FileInfo = domain.FileInfo
