package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (a *App) GetFolderTree() ([]FolderNode, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}
	return buildFolderTree(folders), nil
}

func (a *App) CreateFolderNode(request CreateFolderNodeRequest) (*Folder, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	parentID := strings.TrimSpace(request.ParentID)
	rawPath := strings.TrimSpace(request.Path)
	rawName := strings.TrimSpace(request.Name)

	path := ""
	name := ""
	if rawPath != "" {
		if err := validateFolderPathStrict(rawPath); err != nil {
			return nil, err
		}
		path = normalizeFolderPath(rawPath)
	}
	if rawName != "" {
		if err := validateFolderSegmentStrict(rawName); err != nil {
			return nil, err
		}
		name = normalizeFolderSegment(rawName)
	}

	var (
		folder Folder
		err    error
	)

	switch {
	case path != "":
		folder, err = a.createFolderPath(parentID, path)
	case name != "":
		folder, err = a.db.CreateFolderNode(parentID, name)
	default:
		return nil, fmt.Errorf("folder name or path cannot be empty")
	}
	if err != nil {
		return nil, err
	}

	return &folder, nil
}

func (a *App) createFolderPath(parentID, normalizedPath string) (Folder, error) {
	segments := splitNormalizedFolderPath(normalizedPath)
	if len(segments) == 0 {
		return Folder{}, fmt.Errorf("folder path cannot be empty")
	}

	currentParentID := strings.TrimSpace(parentID)
	var last Folder
	var err error
	for _, segment := range segments {
		last, err = a.db.CreateFolderNode(currentParentID, segment)
		if err != nil {
			return Folder{}, err
		}
		currentParentID = last.ID
	}
	return last, nil
}

func (a *App) DeleteFolderNode(folderID string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return fmt.Errorf("folder id cannot be empty")
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return err
	}
	folderByID := make(map[string]Folder, len(folders))
	childrenByParent := make(map[string][]string, len(folders))
	for _, folder := range folders {
		folderByID[folder.ID] = folder
		parentID := strings.TrimSpace(folder.ParentID)
		childrenByParent[parentID] = append(childrenByParent[parentID], folder.ID)
	}

	target, exists := folderByID[folderID]
	if !exists {
		return fmt.Errorf("folder not found")
	}
	if target.IsSystem {
		return fmt.Errorf("system folder cannot be deleted")
	}

	toDeleteIDs := make([]string, 0, 8)
	var walk func(id string)
	walk = func(id string) {
		toDeleteIDs = append(toDeleteIDs, id)
		for _, childID := range childrenByParent[id] {
			walk(childID)
		}
	}
	walk(folderID)

	sort.SliceStable(toDeleteIDs, func(i, j int) bool {
		left := folderByID[toDeleteIDs[i]]
		right := folderByID[toDeleteIDs[j]]
		leftDepth := len(splitNormalizedFolderPath(left.Path))
		rightDepth := len(splitNormalizedFolderPath(right.Path))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return left.Path > right.Path
	})

	for _, id := range toDeleteIDs {
		papers, err := a.db.GetPapers(id)
		if err != nil {
			return err
		}
		for _, paper := range papers {
			if path := strings.TrimSpace(paper.PDFPath); path != "" {
				_ = os.Remove(path)
			}
			if err := a.db.DeletePaper(paper.ID); err != nil {
				return err
			}
		}
	}

	for _, id := range toDeleteIDs {
		if _, err := a.db.conn.Exec(`UPDATE deepstart_sessions SET target_folder_id = NULL WHERE target_folder_id = ?`, id); err != nil {
			return err
		}
	}

	rootPath := filepath.Join(a.config.DataPath, "papers")
	for _, id := range toDeleteIDs {
		folder := folderByID[id]
		if normalized := normalizeFolderPath(folder.Path); normalized != "" {
			_ = os.RemoveAll(filepath.Join(rootPath, filepath.FromSlash(normalized)))
		}
		_ = os.RemoveAll(filepath.Join(rootPath, folder.ID))
	}

	for _, id := range toDeleteIDs {
		if err := a.db.DeleteFolder(id); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) GetFolderStorageTreeOverview() (*FolderStorageTreeOverview, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}

	statsByFolder := make(map[string]FolderPaperStats, len(folders))
	for _, folder := range folders {
		stats, err := a.db.GetFolderPaperStats(folder.ID)
		if err != nil {
			return nil, err
		}
		statsByFolder[folder.ID] = stats
	}

	rootPath := filepath.Join(a.config.DataPath, "papers")
	folderByID := make(map[string]Folder, len(folders))
	childrenByParent := make(map[string][]Folder, len(folders))
	for _, folder := range folders {
		folderByID[folder.ID] = folder
		parentID := strings.TrimSpace(folder.ParentID)
		childrenByParent[parentID] = append(childrenByParent[parentID], folder)
	}

	sortFolders := func(items []Folder) {
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Path))
			right := strings.ToLower(strings.TrimSpace(items[j].Path))
			if left == right {
				return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
			}
			return left < right
		})
	}
	for key := range childrenByParent {
		list := childrenByParent[key]
		sortFolders(list)
		childrenByParent[key] = list
	}

	var buildNode func(folder Folder) FolderStorageTreeNode
	buildNode = func(folder Folder) FolderStorageTreeNode {
		stats := statsByFolder[folder.ID]
		normalized := normalizeFolderPath(folder.Path)
		if normalized == "" {
			normalized = normalizeFolderPath(folder.Name)
		}
		node := FolderStorageTreeNode{
			FolderID:    folder.ID,
			FolderName:  folder.Name,
			FolderPath:  filepath.Join(rootPath, filepath.FromSlash(normalized)),
			PaperCount:  stats.Total,
			Queued:      stats.Queued,
			Downloading: stats.Downloading,
			Downloaded:  stats.Downloaded,
			Failed:      stats.Failed,
			Children:    []FolderStorageTreeNode{},
		}

		children := childrenByParent[folder.ID]
		for _, child := range children {
			childNode := buildNode(child)
			node.PaperCount += childNode.PaperCount
			node.Queued += childNode.Queued
			node.Downloading += childNode.Downloading
			node.Downloaded += childNode.Downloaded
			node.Failed += childNode.Failed
			node.Children = append(node.Children, childNode)
		}
		return node
	}

	rootFolders := make([]Folder, 0, len(folders))
	for _, folder := range folders {
		parentID := strings.TrimSpace(folder.ParentID)
		if parentID == "" {
			rootFolders = append(rootFolders, folder)
			continue
		}
		if _, exists := folderByID[parentID]; !exists {
			rootFolders = append(rootFolders, folder)
		}
	}
	sortFolders(rootFolders)

	mergedRoots := make([]FolderStorageTreeNode, 0, len(rootFolders))
	for _, root := range rootFolders {
		mergedRoots = append(mergedRoots, buildNode(root))
	}

	return &FolderStorageTreeOverview{
		RootPath:    rootPath,
		Directories: mergedRoots,
		GeneratedAt: time.Now(),
	}, nil
}

func buildFolderTree(folders []Folder) []FolderNode {
	folderByID := make(map[string]Folder, len(folders))
	childrenByParent := make(map[string][]Folder, len(folders))
	for _, folder := range folders {
		folderByID[folder.ID] = folder
		parentID := strings.TrimSpace(folder.ParentID)
		childrenByParent[parentID] = append(childrenByParent[parentID], folder)
	}

	sortFolders := func(items []Folder) {
		sort.SliceStable(items, func(i, j int) bool {
			leftPath := strings.ToLower(strings.TrimSpace(items[i].Path))
			rightPath := strings.ToLower(strings.TrimSpace(items[j].Path))
			if leftPath == rightPath {
				return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
			}
			return leftPath < rightPath
		})
	}
	for key := range childrenByParent {
		list := childrenByParent[key]
		sortFolders(list)
		childrenByParent[key] = list
	}

	var buildNode func(folder Folder) FolderNode
	buildNode = func(folder Folder) FolderNode {
		node := FolderNode{
			Folder:   folder,
			Children: []FolderNode{},
		}
		children := childrenByParent[folder.ID]
		for _, child := range children {
			node.Children = append(node.Children, buildNode(child))
		}
		return node
	}

	rootFolders := make([]Folder, 0, len(folders))
	for _, folder := range folders {
		parentID := strings.TrimSpace(folder.ParentID)
		if parentID == "" {
			rootFolders = append(rootFolders, folder)
			continue
		}
		if _, exists := folderByID[parentID]; !exists {
			rootFolders = append(rootFolders, folder)
		}
	}
	sortFolders(rootFolders)

	result := make([]FolderNode, 0, len(rootFolders))
	for _, root := range rootFolders {
		result = append(result, buildNode(root))
	}
	return result
}
