package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFileAtomicWithWriter(path, mode, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	}, nil)
}

type atomicTempFile struct {
	dir        *os.File
	file       *os.File
	dirPath    string
	tmpName    string
	tmpPath    string
	targetName string
	committed  bool
}

func writeFileAtomicWithWriter(path string, mode os.FileMode, writeFn func(*os.File) error, validateFn func(*os.File, string) error) error {
	if writeFn == nil {
		return fmt.Errorf("atomic file writer cannot be nil")
	}

	tmp, err := newAtomicTempFile(path, mode)
	if err != nil {
		return err
	}
	defer tmp.cleanup()

	if err := tmp.file.Chmod(mode); err != nil {
		return err
	}
	if err := writeFn(tmp.file); err != nil {
		return err
	}
	if err := tmp.file.Sync(); err != nil {
		return err
	}
	if validateFn != nil {
		if _, err := tmp.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := validateFn(tmp.file, tmp.tmpPath); err != nil {
			return err
		}
	}
	if err := tmp.file.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.commit(); err != nil {
		return err
	}
	return nil
}

func newAtomicTempFile(path string, mode os.FileMode) (*atomicTempFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("target path cannot be empty")
	}
	targetName := filepath.Base(filepath.Clean(path))
	if targetName == "." || targetName == string(os.PathSeparator) || strings.Contains(targetName, string(os.PathSeparator)) {
		return nil, fmt.Errorf("target file name is invalid")
	}

	dirPath := filepath.Dir(path)
	if err := ensurePlainDirectory(dirPath); err != nil {
		return nil, err
	}
	dir, cleanDir, err := openPlainDirectoryNoFollow(dirPath)
	if err != nil {
		return nil, err
	}

	prefix := "." + targetName + ".tmp-"
	for attempts := 0; attempts < 100; attempts++ {
		tmpName, err := randomAtomicTempName(prefix)
		if err != nil {
			_ = dir.Close()
			return nil, err
		}
		fd, err := unix.Openat(int(dir.Fd()), tmpName, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, uint32(mode.Perm()))
		if err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			_ = dir.Close()
			return nil, err
		}
		file := os.NewFile(uintptr(fd), filepath.Join(cleanDir, tmpName))
		if file == nil {
			_ = unix.Close(fd)
			_ = dir.Close()
			return nil, fmt.Errorf("failed to wrap atomic temp file")
		}
		return &atomicTempFile{
			dir:        dir,
			file:       file,
			dirPath:    cleanDir,
			tmpName:    tmpName,
			tmpPath:    filepath.Join(cleanDir, tmpName),
			targetName: targetName,
		}, nil
	}
	_ = dir.Close()
	return nil, fmt.Errorf("failed to create unique atomic temp file")
}

func randomAtomicTempName(prefix string) (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(randomBytes[:]), nil
}

func (tmp *atomicTempFile) commit() error {
	if tmp == nil || tmp.dir == nil {
		return fmt.Errorf("atomic temp file is not open")
	}
	if err := tmp.ensureDirectoryPathStillOpen(); err != nil {
		return err
	}
	if tmp.file != nil {
		if err := tmp.file.Close(); err != nil {
			tmp.file = nil
			return err
		}
		tmp.file = nil
	}

	if err := unix.Renameat(int(tmp.dir.Fd()), tmp.tmpName, int(tmp.dir.Fd()), tmp.targetName); err != nil {
		return err
	}
	tmp.committed = true

	if err := tmp.ensureDirectoryPathStillOpen(); err != nil {
		_ = unix.Unlinkat(int(tmp.dir.Fd()), tmp.targetName, 0)
		_ = syncDirectoryHandle(tmp.dir)
		return err
	}
	if err := syncDirectoryHandle(tmp.dir); err != nil {
		return err
	}
	return nil
}

func (tmp *atomicTempFile) syncExistingTempFile(mode os.FileMode) error {
	file, err := tmp.openExistingTempFile()
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("atomic temp file is empty or invalid")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return tmp.ensureDirectoryPathStillOpen()
}

func (tmp *atomicTempFile) openExistingTempFile() (*os.File, error) {
	if tmp == nil || tmp.dir == nil || tmp.tmpName == "" {
		return nil, fmt.Errorf("atomic temp file is not open")
	}
	if err := tmp.ensureDirectoryPathStillOpen(); err != nil {
		return nil, err
	}
	fd, err := unix.Openat(int(tmp.dir.Fd()), tmp.tmpName, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), tmp.tmpPath)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed to wrap atomic temp file")
	}
	return file, nil
}

func (tmp *atomicTempFile) cleanup() {
	if tmp == nil {
		return
	}
	if tmp.file != nil {
		_ = tmp.file.Close()
		tmp.file = nil
	}
	if tmp.dir != nil {
		if !tmp.committed && tmp.tmpName != "" {
			_ = unix.Unlinkat(int(tmp.dir.Fd()), tmp.tmpName, 0)
		}
		_ = tmp.dir.Close()
		tmp.dir = nil
	}
}

func (tmp *atomicTempFile) ensureDirectoryPathStillOpen() error {
	if tmp == nil || tmp.dir == nil {
		return fmt.Errorf("target directory is not open")
	}
	return ensureOpenedDirectoryPathStillOpen(tmp.dir, tmp.dirPath, "target directory changed while writing")
}

func renameFileAtomicWithinDir(sourcePath, targetPath string) error {
	sourcePath = filepath.Clean(strings.TrimSpace(sourcePath))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if sourcePath == "" || targetPath == "" {
		return fmt.Errorf("source and target paths cannot be empty")
	}
	sourceDir := filepath.Dir(sourcePath)
	targetDir := filepath.Dir(targetPath)
	if sourceDir != targetDir {
		return fmt.Errorf("source and target must be in the same directory")
	}
	sourceName := filepath.Base(sourcePath)
	targetName := filepath.Base(targetPath)
	if sourceName == "." || targetName == "." {
		return fmt.Errorf("source or target file name is invalid")
	}
	if err := ensurePlainDirectory(targetDir); err != nil {
		return err
	}
	dir, cleanDir, err := openPlainDirectoryNoFollow(targetDir)
	if err != nil {
		return err
	}
	defer dir.Close()

	if err := ensureOpenedDirectoryPathStillOpen(dir, cleanDir, "target directory changed while replacing"); err != nil {
		return err
	}
	if err := unix.Renameat(int(dir.Fd()), sourceName, int(dir.Fd()), targetName); err != nil {
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(dir, cleanDir, "target directory changed while replacing"); err != nil {
		_ = unix.Unlinkat(int(dir.Fd()), targetName, 0)
		_ = syncDirectoryHandle(dir)
		return err
	}
	return syncDirectoryHandle(dir)
}

func renameFileAtomicAcrossDirs(sourcePath, targetPath string) error {
	sourcePath = filepath.Clean(strings.TrimSpace(sourcePath))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if sourcePath == "" || targetPath == "" {
		return fmt.Errorf("source and target paths cannot be empty")
	}
	sourceDir := filepath.Dir(sourcePath)
	targetDir := filepath.Dir(targetPath)
	if sourceDir == targetDir {
		return renameFileAtomicWithinDir(sourcePath, targetPath)
	}

	sourceName := filepath.Base(sourcePath)
	targetName := filepath.Base(targetPath)
	if sourceName == "." || targetName == "." {
		return fmt.Errorf("source or target file name is invalid")
	}
	if err := ensurePlainDirectory(targetDir); err != nil {
		return err
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("target path already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	sourceHandle, cleanSourceDir, err := openPlainDirectoryNoFollow(sourceDir)
	if err != nil {
		return err
	}
	defer sourceHandle.Close()
	targetHandle, cleanTargetDir, err := openPlainDirectoryNoFollow(targetDir)
	if err != nil {
		return err
	}
	defer targetHandle.Close()

	if err := ensureOpenedDirectoryPathStillOpen(sourceHandle, cleanSourceDir, "source directory changed while moving"); err != nil {
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(targetHandle, cleanTargetDir, "target directory changed while moving"); err != nil {
		return err
	}
	if err := unix.Renameat(int(sourceHandle.Fd()), sourceName, int(targetHandle.Fd()), targetName); err != nil {
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(sourceHandle, cleanSourceDir, "source directory changed while moving"); err != nil {
		_ = unix.Renameat(int(targetHandle.Fd()), targetName, int(sourceHandle.Fd()), sourceName)
		_ = syncDirectoryHandle(sourceHandle)
		_ = syncDirectoryHandle(targetHandle)
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(targetHandle, cleanTargetDir, "target directory changed while moving"); err != nil {
		_ = unix.Renameat(int(targetHandle.Fd()), targetName, int(sourceHandle.Fd()), sourceName)
		_ = syncDirectoryHandle(sourceHandle)
		_ = syncDirectoryHandle(targetHandle)
		return err
	}
	if err := syncDirectoryHandle(sourceHandle); err != nil {
		return err
	}
	return syncDirectoryHandle(targetHandle)
}

func ensureOpenedDirectoryPathStillOpen(dir *os.File, dirPath string, message string) error {
	if dir == nil {
		return fmt.Errorf("%s", message)
	}
	currentInfo, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	openInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(currentInfo, openInfo) {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func openPlainDirectoryNoFollow(dir string) (*os.File, string, error) {
	cleanDir, err := cleanPathForNoFollowOpen(dir)
	if err != nil {
		return nil, "", err
	}
	if !filepath.IsAbs(cleanDir) {
		return nil, "", fmt.Errorf("directory path must be absolute")
	}

	root := string(os.PathSeparator)
	currentFD, err := unix.Open(root, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, "", err
	}
	rest := strings.TrimPrefix(cleanDir, root)
	if rest == "" {
		file := os.NewFile(uintptr(currentFD), root)
		if file == nil {
			_ = unix.Close(currentFD)
			return nil, "", fmt.Errorf("failed to wrap root directory")
		}
		return file, root, nil
	}

	for _, part := range strings.Split(rest, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		nextFD, err := unix.Openat(currentFD, part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		_ = unix.Close(currentFD)
		if err != nil {
			return nil, "", err
		}
		currentFD = nextFD
	}

	file := os.NewFile(uintptr(currentFD), cleanDir)
	if file == nil {
		_ = unix.Close(currentFD)
		return nil, "", fmt.Errorf("failed to wrap target directory")
	}
	return file, cleanDir, nil
}

func cleanPathForNoFollowOpen(path string) (string, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		switch {
		case cleanPath == "/var":
			return "/private/var", nil
		case strings.HasPrefix(cleanPath, "/var/"):
			return filepath.Join("/private/var", strings.TrimPrefix(cleanPath, "/var/")), nil
		case cleanPath == "/tmp":
			return "/private/tmp", nil
		case strings.HasPrefix(cleanPath, "/tmp/"):
			return filepath.Join("/private/tmp", strings.TrimPrefix(cleanPath, "/tmp/")), nil
		}
	}
	return cleanPath, nil
}

func syncDirectory(dir string) error {
	handle, _, err := openPlainDirectoryNoFollow(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return syncDirectoryHandle(handle)
}

func syncDirectoryHandle(handle *os.File) error {
	if handle == nil {
		return fmt.Errorf("directory handle cannot be nil")
	}
	if err := handle.Sync(); err != nil {
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
			return nil
		}
		return err
	}
	return nil
}

func syncFile(path string) error {
	handle, err := os.Open(path)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}

func removeFileAndSyncDir(path string) error {
	return removePathWithParentDir(path, 0)
}

func removeDirectoryAndSyncParent(path string) error {
	return removePathWithParentDir(path, unix.AT_REMOVEDIR)
}

func removeTreeAndSyncParent(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	dirPath := filepath.Dir(path)
	entryName := filepath.Base(path)
	if entryName == "." || entryName == string(os.PathSeparator) {
		return fmt.Errorf("tree name is invalid")
	}
	parent, cleanDir, err := openPlainDirectoryNoFollow(dirPath)
	if err != nil {
		return err
	}
	defer parent.Close()
	if err := ensureOpenedDirectoryPathStillOpen(parent, cleanDir, "target directory changed while removing tree"); err != nil {
		return err
	}
	if err := removeTreeEntryAt(parent, entryName); err != nil {
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(parent, cleanDir, "target directory changed while removing tree"); err != nil {
		return err
	}
	return syncDirectoryHandle(parent)
}

func removeTreeEntryAt(parent *os.File, name string) error {
	if parent == nil {
		return fmt.Errorf("parent directory handle cannot be nil")
	}
	if name == "" || name == "." || name == ".." || strings.Contains(name, string(os.PathSeparator)) {
		return fmt.Errorf("tree entry name is invalid")
	}
	parentFD := int(parent.Fd())
	if err := unix.Unlinkat(parentFD, name, 0); err == nil || errors.Is(err, unix.ENOENT) {
		return nil
	} else if !errors.Is(err, unix.EISDIR) && !errors.Is(err, unix.EPERM) {
		return err
	}

	childFD, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	child := os.NewFile(uintptr(childFD), name)
	if child == nil {
		_ = unix.Close(childFD)
		return fmt.Errorf("failed to wrap child directory")
	}
	defer child.Close()

	childStat, err := unixFileStat(child)
	if err != nil {
		return err
	}
	if err := removeTreeContentsAt(child); err != nil {
		return err
	}
	if err := syncDirectoryHandle(child); err != nil {
		return err
	}
	currentStat, err := unixEntryStatNoFollow(parentFD, name)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	if !sameUnixFileStat(childStat, currentStat) {
		return fmt.Errorf("tree entry changed while removing")
	}
	if err := unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	return nil
}

func removeTreeContentsAt(dir *os.File) error {
	for {
		names, err := dir.Readdirnames(128)
		for _, name := range names {
			if name == "." || name == ".." {
				continue
			}
			if err := removeTreeEntryAt(dir, name); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func unixFileStat(file *os.File) (unix.Stat_t, error) {
	var stat unix.Stat_t
	if file == nil {
		return stat, fmt.Errorf("file handle cannot be nil")
	}
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return stat, err
	}
	return stat, nil
}

func unixEntryStatNoFollow(parentFD int, name string) (unix.Stat_t, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return stat, err
	}
	return stat, nil
}

func sameUnixFileStat(left, right unix.Stat_t) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino
}

func removePathWithParentDir(path string, flags int) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	if fileName == "." || fileName == string(os.PathSeparator) {
		return fmt.Errorf("file name is invalid")
	}
	dir, cleanDir, err := openPlainDirectoryNoFollow(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := ensureOpenedDirectoryPathStillOpen(dir, cleanDir, "target directory changed while removing"); err != nil {
		return err
	}
	if err := unix.Unlinkat(int(dir.Fd()), fileName, flags); err != nil {
		return err
	}
	if err := ensureOpenedDirectoryPathStillOpen(dir, cleanDir, "target directory changed while removing"); err != nil {
		return err
	}
	return syncDirectoryHandle(dir)
}

func ensurePlainDirectory(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "."
	}
	cleanDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return err
	}

	volume := filepath.VolumeName(cleanDir)
	rest := strings.TrimPrefix(cleanDir, volume)
	current := volume
	if filepath.IsAbs(cleanDir) {
		current = volume + string(os.PathSeparator)
		rest = strings.TrimPrefix(rest, string(os.PathSeparator))
	}
	if current == "" {
		current = "."
	}

	for _, part := range strings.Split(rest, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := os.Mkdir(current, 0700); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = os.Lstat(current)
			if err != nil {
				return err
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if isAllowedSystemDirectorySymlink(current) {
				continue
			}
			if filepath.Clean(current) == cleanDir {
				return fmt.Errorf("target directory is a symbolic link")
			}
			return fmt.Errorf("target directory contains symbolic link")
		}
		if !info.IsDir() {
			return fmt.Errorf("target path is not a directory")
		}
	}
	return nil
}

func ensurePathHasNoSymlinkComponents(path string, label string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		label = "path"
	}

	volume := filepath.VolumeName(cleanPath)
	rest := strings.TrimPrefix(cleanPath, volume)
	current := volume
	if filepath.IsAbs(cleanPath) {
		current = volume + string(os.PathSeparator)
		rest = strings.TrimPrefix(rest, string(os.PathSeparator))
	}
	if current == "" {
		current = "."
	}

	for _, part := range strings.Split(rest, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if isAllowedSystemDirectorySymlink(current) {
				continue
			}
			if filepath.Clean(current) == cleanPath {
				return fmt.Errorf("%s is a symbolic link", label)
			}
			return fmt.Errorf("%s contains symbolic link", label)
		}
		if !info.IsDir() && filepath.Clean(current) != cleanPath {
			return fmt.Errorf("%s contains non-directory path", label)
		}
	}
	return nil
}

func isAllowedSystemDirectorySymlink(path string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	cleaned := filepath.Clean(path)
	if cleaned != "/var" && cleaned != "/tmp" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return false
	}
	return resolved == "/private/var" || resolved == "/private/tmp"
}

func removeDirectChildIfPlainRoot(rootPath, childPath string) error {
	rootPath = filepath.Clean(rootPath)
	childPath = filepath.Clean(childPath)
	managedChildPath, ok := managedPathInsideRoot(rootPath, childPath)
	if !ok {
		return nil
	}

	rootAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed cleanup root is a symbolic link")
	}
	if !info.IsDir() {
		return fmt.Errorf("managed cleanup root is not a directory")
	}

	rel, err := filepath.Rel(rootAbs, managedChildPath)
	if err != nil {
		return err
	}
	if rel == "." || rel == "" || filepath.IsAbs(rel) || rel == ".." || filepath.Dir(rel) != "." {
		return fmt.Errorf("managed cleanup path must be a direct child")
	}
	return removeTreeAndSyncParent(managedChildPath)
}
