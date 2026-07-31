package app

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

func (a *App) RenameFolderNode(request RenameFolderNodeRequest) (*Folder, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folderID := strings.TrimSpace(request.FolderID)
	if folderID == "" {
		return nil, fmt.Errorf("folder id cannot be empty")
	}
	if err := validateFolderSegmentStrict(request.Name); err != nil {
		return nil, err
	}
	newName := normalizeFolderSegment(request.Name)
	if newName == "" {
		return nil, fmt.Errorf("folder name cannot be empty")
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}
	folderByID := make(map[string]Folder, len(folders))
	for _, folder := range folders {
		folderByID[folder.ID] = folder
	}

	target, exists := folderByID[folderID]
	if !exists {
		return nil, fmt.Errorf("folder not found")
	}
	if target.IsSystem {
		return nil, fmt.Errorf("system folder cannot be renamed")
	}

	oldPath := normalizeFolderPath(target.Path)
	if oldPath == "" {
		oldPath = normalizeFolderPath(target.Name)
	}
	parentPath := ""
	if parentID := strings.TrimSpace(target.ParentID); parentID != "" {
		parent, exists := folderByID[parentID]
		if !exists {
			return nil, fmt.Errorf("parent folder not found")
		}
		parentPath = normalizeFolderPath(parent.Path)
	}
	newPath := normalizeFolderPath(newName)
	if parentPath != "" {
		newPath = normalizeFolderPath(parentPath + "/" + newName)
	}
	if newPath == "" {
		return nil, fmt.Errorf("folder path cannot be empty")
	}
	if oldPath == newPath && target.Name == newName {
		current := target
		return &current, nil
	}

	for _, folder := range folders {
		if folder.ID == target.ID {
			continue
		}
		if normalizeFolderPath(folder.Path) == newPath {
			return nil, fmt.Errorf("folder path already exists")
		}
	}

	renamedFolders := foldersAffectedByRename(folders, target.ID, oldPath, newPath, newName)
	paperPathUpdates, err := a.paperPathUpdatesForFolderRename(renamedFolders, oldPath, newPath)
	if err != nil {
		return nil, err
	}

	movedDirectory := false
	oldDir, newDir, err := a.moveManagedFolderDirectoryForRename(oldPath, newPath)
	if err != nil {
		return nil, err
	}
	movedDirectory = oldDir != "" && newDir != ""

	tx, err := a.db.conn.Begin()
	if err != nil {
		if movedDirectory {
			_ = renameFileAtomicWithinDir(newDir, oldDir)
		}
		return nil, err
	}
	defer tx.Rollback()

	for _, folder := range renamedFolders {
		if _, err := tx.Exec(`UPDATE folders SET name = ?, path = ? WHERE id = ?`, folder.Name, folder.Path, folder.ID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicWithinDir(newDir, oldDir)
			}
			return nil, err
		}
	}
	for _, update := range paperPathUpdates {
		if _, err := tx.Exec(`UPDATE papers SET pdf_path = ?, updated_at = ? WHERE id = ?`, update.NewPath, time.Now(), update.PaperID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicWithinDir(newDir, oldDir)
			}
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE deepread_parse_cache SET pdf_path = ? WHERE paper_id = ?`, update.NewPath, update.PaperID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicWithinDir(newDir, oldDir)
			}
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		if movedDirectory {
			_ = renameFileAtomicWithinDir(newDir, oldDir)
		}
		return nil, err
	}

	renamed := renamedFolders[0]
	return &renamed, nil
}

func (a *App) MoveFolderNode(request MoveFolderNodeRequest) (*Folder, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}

	folderID := strings.TrimSpace(request.FolderID)
	targetParentID := strings.TrimSpace(request.ParentID)
	if folderID == "" {
		return nil, fmt.Errorf("folder id cannot be empty")
	}
	if folderID == targetParentID {
		return nil, fmt.Errorf("folder cannot be moved into itself")
	}

	folders, err := a.db.GetFolders()
	if err != nil {
		return nil, err
	}
	folderByID := make(map[string]Folder, len(folders))
	for _, folder := range folders {
		folderByID[folder.ID] = folder
	}

	target, exists := folderByID[folderID]
	if !exists {
		return nil, fmt.Errorf("folder not found")
	}
	if target.IsSystem {
		return nil, fmt.Errorf("system folder cannot be moved")
	}

	oldPath := normalizeFolderPath(target.Path)
	if oldPath == "" {
		oldPath = normalizeFolderPath(target.Name)
	}
	targetName := normalizeFolderSegment(target.Name)
	parentPath := ""
	if targetParentID != "" {
		parent, exists := folderByID[targetParentID]
		if !exists {
			return nil, fmt.Errorf("target parent folder not found")
		}
		parentPath = normalizeFolderPath(parent.Path)
		if parentPath == oldPath || strings.HasPrefix(parentPath, oldPath+"/") {
			return nil, fmt.Errorf("folder cannot be moved into its descendant")
		}
	}

	newPath := targetName
	if parentPath != "" {
		newPath = normalizeFolderPath(parentPath + "/" + targetName)
	}
	if newPath == "" {
		return nil, fmt.Errorf("folder path cannot be empty")
	}
	if targetParentID == strings.TrimSpace(target.ParentID) && oldPath == newPath {
		current := target
		return &current, nil
	}
	for _, folder := range folders {
		if folder.ID == target.ID {
			continue
		}
		if normalizeFolderPath(folder.Path) == newPath {
			return nil, fmt.Errorf("folder path already exists")
		}
	}

	movedFolders := foldersAffectedByRename(folders, target.ID, oldPath, newPath, targetName)
	for i := range movedFolders {
		if movedFolders[i].ID == target.ID {
			movedFolders[i].ParentID = targetParentID
			break
		}
	}
	paperPathUpdates, err := a.paperPathUpdatesForFolderRename(movedFolders, oldPath, newPath)
	if err != nil {
		return nil, err
	}

	movedDirectory := false
	oldDir, newDir, err := a.moveManagedFolderDirectoryForMove(oldPath, newPath)
	if err != nil {
		return nil, err
	}
	movedDirectory = oldDir != "" && newDir != ""

	tx, err := a.db.conn.Begin()
	if err != nil {
		if movedDirectory {
			_ = renameFileAtomicAcrossDirs(newDir, oldDir)
		}
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, folder := range movedFolders {
		if folder.ID == target.ID {
			if _, err := tx.Exec(`UPDATE folders SET parent_id = ?, name = ?, path = ? WHERE id = ?`, nullIfBlank(folder.ParentID), folder.Name, folder.Path, folder.ID); err != nil {
				if movedDirectory {
					_ = renameFileAtomicAcrossDirs(newDir, oldDir)
				}
				return nil, err
			}
			continue
		}
		if _, err := tx.Exec(`UPDATE folders SET name = ?, path = ? WHERE id = ?`, folder.Name, folder.Path, folder.ID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicAcrossDirs(newDir, oldDir)
			}
			return nil, err
		}
	}
	for _, update := range paperPathUpdates {
		if _, err := tx.Exec(`UPDATE papers SET pdf_path = ?, updated_at = ? WHERE id = ?`, update.NewPath, now, update.PaperID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicAcrossDirs(newDir, oldDir)
			}
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE deepread_parse_cache SET pdf_path = ? WHERE paper_id = ?`, update.NewPath, update.PaperID); err != nil {
			if movedDirectory {
				_ = renameFileAtomicAcrossDirs(newDir, oldDir)
			}
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		if movedDirectory {
			_ = renameFileAtomicAcrossDirs(newDir, oldDir)
		}
		return nil, err
	}

	moved := movedFolders[0]
	return &moved, nil
}

type folderRenamePaperPathUpdate struct {
	PaperID string
	OldPath string
	NewPath string
}

func foldersAffectedByRename(folders []Folder, targetID, oldPath, newPath, newName string) []Folder {
	affected := make([]Folder, 0, len(folders))
	for _, folder := range folders {
		path := normalizeFolderPath(folder.Path)
		if folder.ID != targetID && path != oldPath && !strings.HasPrefix(path, oldPath+"/") {
			continue
		}
		next := folder
		if folder.ID == targetID {
			next.Name = newName
			next.Path = newPath
		} else {
			suffix := strings.TrimPrefix(path, oldPath)
			suffix = strings.TrimPrefix(suffix, "/")
			next.Path = normalizeFolderPath(newPath + "/" + suffix)
		}
		affected = append(affected, next)
	}
	sort.SliceStable(affected, func(i, j int) bool {
		if affected[i].ID == targetID {
			return true
		}
		if affected[j].ID == targetID {
			return false
		}
		return affected[i].Path < affected[j].Path
	})
	return affected
}

func (a *App) paperPathUpdatesForFolderRename(renamedFolders []Folder, oldPath, newPath string) ([]folderRenamePaperPathUpdate, error) {
	rootPath := filepath.Join(a.config.DataPath, "papers")
	oldFolderPath := filepath.Join(rootPath, filepath.FromSlash(oldPath))
	newFolderPath := filepath.Join(rootPath, filepath.FromSlash(newPath))
	updates := []folderRenamePaperPathUpdate{}

	for _, folder := range renamedFolders {
		papers, err := a.db.GetPapers(folder.ID)
		if err != nil {
			return nil, err
		}
		for _, paper := range papers {
			pdfPath := strings.TrimSpace(paper.PDFPath)
			if pdfPath == "" {
				continue
			}
			managedPath, ok := managedPathInsideRoot(rootPath, pdfPath)
			if !ok {
				continue
			}
			rel, err := filepath.Rel(oldFolderPath, managedPath)
			if err != nil || rel == "." || rel == "" || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				continue
			}
			updates = append(updates, folderRenamePaperPathUpdate{
				PaperID: paper.ID,
				OldPath: managedPath,
				NewPath: filepath.Join(newFolderPath, rel),
			})
		}
	}
	return updates, nil
}

func (a *App) moveManagedFolderDirectoryForRename(oldPath, newPath string) (string, string, error) {
	rootPath := filepath.Join(a.config.DataPath, "papers")
	oldDir := filepath.Join(rootPath, filepath.FromSlash(oldPath))
	newDir := filepath.Join(rootPath, filepath.FromSlash(newPath))

	oldManagedDir, ok := managedPathInsideRoot(rootPath, oldDir)
	if !ok {
		return "", "", fmt.Errorf("old folder path is outside managed papers root")
	}
	newManagedDir, ok := managedPathInsideRoot(rootPath, newDir)
	if !ok {
		return "", "", fmt.Errorf("new folder path is outside managed papers root")
	}
	if filepath.Dir(oldManagedDir) != filepath.Dir(newManagedDir) {
		return "", "", fmt.Errorf("folder rename must stay within the same parent directory")
	}

	if _, err := os.Lstat(newManagedDir); err == nil {
		return "", "", fmt.Errorf("target folder directory already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	if _, err := os.Lstat(oldManagedDir); err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	if _, err := managedPathWithPlainExistingParent(rootPath, oldManagedDir); err != nil {
		return "", "", err
	}
	if _, err := managedPathWithPlainExistingParent(rootPath, newManagedDir); err != nil {
		return "", "", err
	}
	if err := renameFileAtomicWithinDir(oldManagedDir, newManagedDir); err != nil {
		return "", "", err
	}
	return oldManagedDir, newManagedDir, nil
}

func (a *App) moveManagedFolderDirectoryForMove(oldPath, newPath string) (string, string, error) {
	rootPath := filepath.Join(a.config.DataPath, "papers")
	oldDir := filepath.Join(rootPath, filepath.FromSlash(oldPath))
	newDir := filepath.Join(rootPath, filepath.FromSlash(newPath))

	oldManagedDir, ok := managedPathInsideRoot(rootPath, oldDir)
	if !ok {
		return "", "", fmt.Errorf("old folder path is outside managed papers root")
	}
	newManagedDir, ok := managedPathInsideRoot(rootPath, newDir)
	if !ok {
		return "", "", fmt.Errorf("new folder path is outside managed papers root")
	}
	if _, err := os.Lstat(newManagedDir); err == nil {
		return "", "", fmt.Errorf("target folder directory already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}
	if _, err := os.Lstat(oldManagedDir); err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	if _, err := managedPathWithPlainExistingParent(rootPath, oldManagedDir); err != nil {
		return "", "", err
	}
	newParentProbe := filepath.Join(filepath.Dir(newManagedDir), ".folder-move-parent")
	if _, err := ensureManagedFileParent(rootPath, newParentProbe); err != nil {
		return "", "", err
	}
	if err := renameFileAtomicAcrossDirs(oldManagedDir, newManagedDir); err != nil {
		return "", "", err
	}
	return oldManagedDir, newManagedDir, nil
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
				if err := removeManagedFileIfPresent(a.config.DataPath, path); err != nil {
					return err
				}
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
			if err := removeManagedPathIfPresent(rootPath, filepath.Join(rootPath, filepath.FromSlash(normalized))); err != nil {
				return err
			}
		}
		if err := removeManagedPathIfPresent(rootPath, filepath.Join(rootPath, folder.ID)); err != nil {
			return err
		}
	}

	for _, id := range toDeleteIDs {
		if err := a.db.DeleteFolder(id); err != nil {
			return err
		}
	}

	return nil
}

func removeManagedFileIfPresent(dataPath, candidatePath string) error {
	rootPath := filepath.Join(dataPath, "papers")
	managedPath, ok := managedPathInsideRoot(rootPath, candidatePath)
	if !ok {
		return nil
	}
	managedPath, err := managedPathWithPlainExistingParent(rootPath, managedPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(managedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	return removeFileAndSyncDir(managedPath)
}

func removeManagedPathIfPresent(rootPath, candidatePath string) error {
	managedPath, ok := managedPathInsideRoot(rootPath, candidatePath)
	if !ok {
		return nil
	}
	managedPath, err := managedPathWithPlainExistingParent(rootPath, managedPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(managedPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return removeTreeAndSyncParent(managedPath)
}

func managedPathWithPlainExistingParent(rootPath, candidatePath string) (string, error) {
	managedPath, ok := managedPathInsideRoot(rootPath, candidatePath)
	if !ok {
		return "", fmt.Errorf("managed path is outside root")
	}

	rootAbs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootPath)))
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return managedPath, nil
		}
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("managed root directory is a symbolic link")
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("managed root path is not a directory")
	}

	parent := filepath.Dir(managedPath)
	rel, err := filepath.Rel(rootAbs, parent)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == "" {
		return managedPath, nil
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("managed path parent is outside root")
	}

	current := rootAbs
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return managedPath, nil
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed path parent contains symbolic link")
		}
		if !info.IsDir() {
			return "", fmt.Errorf("managed path parent contains non-directory path")
		}
	}

	return managedPath, nil
}

func managedPathInsideRoot(rootPath, candidatePath string) (string, bool) {
	rootPath = strings.TrimSpace(rootPath)
	candidatePath = strings.TrimSpace(candidatePath)
	if rootPath == "" || candidatePath == "" {
		return "", false
	}

	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", false
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidatePath))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return candidateAbs, true
}

func ensureManagedFileParent(rootPath, candidatePath string) (string, error) {
	managedPath, ok := managedPathInsideRoot(rootPath, candidatePath)
	if !ok {
		return "", fmt.Errorf("managed file path is outside root")
	}

	rootAbs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootPath)))
	if err != nil {
		return "", err
	}
	if err := ensurePlainDirectory(rootAbs); err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(rootAbs)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("managed root directory is a symbolic link")
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("managed root path is not a directory")
	}

	parent := filepath.Dir(managedPath)
	rel, err := filepath.Rel(rootAbs, parent)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == "" {
		return managedPath, nil
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("managed file parent is outside root")
	}

	current := rootAbs
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				if mkdirErr := os.Mkdir(current, 0700); mkdirErr != nil {
					return "", mkdirErr
				}
				info, err = os.Lstat(current)
				if err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed file parent contains symbolic link")
		}
		if !info.IsDir() {
			return "", fmt.Errorf("managed file parent contains non-directory path")
		}
	}

	return managedPath, nil
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
