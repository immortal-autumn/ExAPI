//go:build linux

package privatecutover

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const secureBackupOpenFlags = unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK

func secureSnapshotBackupTree(root backupRootIdentity, _ time.Time) ([]backupCandidateIdentity, error) {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootDirectory.Close()

	var candidates []backupCandidateIdentity
	if err := walkSecureBackupDirectory(root, rootDirectory, "", &candidates); err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].Path < candidates[right].Path })
	return candidates, nil
}

func secureValidateBackupRoot(root backupRootIdentity) error {
	directory, err := openSecureBackupRoot(root)
	if err != nil {
		return err
	}
	return directory.Close()
}

func walkSecureBackupDirectory(root backupRootIdentity, directory *os.File, relativeDirectory string, candidates *[]backupCandidateIdentity) error {
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("read legacy local backup directory %s: %w", filepath.Join(root.Path, relativeDirectory), err)
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." || strings.ContainsRune(entry.Name(), os.PathSeparator) {
			return fmt.Errorf("legacy local backup tree contains an unsafe entry name: %q", entry.Name())
		}
		relativePath := entry.Name()
		if relativeDirectory != "" {
			relativePath = filepath.Join(relativeDirectory, entry.Name())
		}
		absolutePath := filepath.Join(root.Path, relativePath)
		childFD, openErr := unix.Openat(int(directory.Fd()), entry.Name(), secureBackupOpenFlags, 0)
		if errors.Is(openErr, unix.ELOOP) {
			return fmt.Errorf("legacy local backup tree contains a symlink: %s", absolutePath)
		}
		if openErr != nil {
			return &os.PathError{Op: "open legacy local backup entry without symlinks", Path: absolutePath, Err: openErr}
		}
		child := os.NewFile(uintptr(childFD), absolutePath)
		if child == nil {
			_ = unix.Close(childFD)
			return fmt.Errorf("open legacy local backup entry %s: invalid file descriptor", absolutePath)
		}
		info, statErr := child.Stat()
		if statErr != nil {
			_ = child.Close()
			return fmt.Errorf("stat legacy local backup entry %s: %w", absolutePath, statErr)
		}
		if info.IsDir() {
			err = walkSecureBackupDirectory(root, child, relativePath, candidates)
			_ = child.Close()
			if err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			_ = child.Close()
			return fmt.Errorf("legacy local backup tree contains a non-regular file: %s", absolutePath)
		}
		candidate, snapshotErr := snapshotOpenedBackupCandidate(child, absolutePath, relativePath)
		_ = child.Close()
		if snapshotErr != nil {
			return snapshotErr
		}
		// The descriptor-rooted snapshot itself proves that the file was present
		// before the destructive transaction. Filesystem mtimes are operator- and
		// application-controlled metadata; using them as an eligibility filter can
		// silently leave a future-dated or timestamp-rounded backup behind.
		*candidates = append(*candidates, candidate)
	}
	return nil
}

// secureAssertBackupTreeHasNoFiles performs a fresh descriptor-rooted walk
// after every committed candidate has been durably unlinked. Empty directories
// may remain, but any file, symlink, or special entry means the offline backup
// root changed after the committed snapshot and finalization must fail closed.
func secureAssertBackupTreeHasNoFiles(root backupRootIdentity) error {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return err
	}
	defer rootDirectory.Close()
	return assertSecureBackupDirectoryHasNoFiles(root, rootDirectory, "")
}

func assertSecureBackupDirectoryHasNoFiles(root backupRootIdentity, directory *os.File, relativeDirectory string) error {
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("re-scan legacy local backup directory %s: %w", filepath.Join(root.Path, relativeDirectory), err)
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." || strings.ContainsRune(entry.Name(), os.PathSeparator) {
			return fmt.Errorf("legacy local backup tree contains an unsafe entry name during final verification: %q", entry.Name())
		}
		relativePath := entry.Name()
		if relativeDirectory != "" {
			relativePath = filepath.Join(relativeDirectory, entry.Name())
		}
		absolutePath := filepath.Join(root.Path, relativePath)
		childFD, openErr := unix.Openat(int(directory.Fd()), entry.Name(), secureBackupOpenFlags, 0)
		if errors.Is(openErr, unix.ELOOP) {
			return fmt.Errorf("legacy local backup tree contains a symlink during final verification: %s", absolutePath)
		}
		if openErr != nil {
			return &os.PathError{Op: "open legacy local backup entry during final verification", Path: absolutePath, Err: openErr}
		}
		child := os.NewFile(uintptr(childFD), absolutePath)
		if child == nil {
			_ = unix.Close(childFD)
			return fmt.Errorf("open legacy local backup entry during final verification %s: invalid file descriptor", absolutePath)
		}
		info, statErr := child.Stat()
		if statErr != nil {
			_ = child.Close()
			return fmt.Errorf("stat legacy local backup entry during final verification %s: %w", absolutePath, statErr)
		}
		if info.IsDir() {
			err = assertSecureBackupDirectoryHasNoFiles(root, child, relativePath)
			_ = child.Close()
			if err != nil {
				return err
			}
			continue
		}
		_ = child.Close()
		return fmt.Errorf("legacy local backup tree still contains an uncommitted entry after purge: %s", absolutePath)
	}
	return nil
}

func snapshotOpenedBackupCandidate(file *os.File, absolutePath, relativePath string) (backupCandidateIdentity, error) {
	before, err := file.Stat()
	if err != nil {
		return backupCandidateIdentity{}, fmt.Errorf("stat legacy local backup candidate %s: %w", absolutePath, err)
	}
	metadata, err := backupCandidateMetadataFromInfo(absolutePath, before)
	if err != nil {
		return backupCandidateIdentity{}, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return backupCandidateIdentity{}, fmt.Errorf("hash legacy local backup candidate %s: %w", absolutePath, err)
	}
	after, err := file.Stat()
	if err != nil {
		return backupCandidateIdentity{}, fmt.Errorf("restat legacy local backup candidate %s: %w", absolutePath, err)
	}
	afterMetadata, err := backupCandidateMetadataFromInfo(absolutePath, after)
	if err != nil {
		return backupCandidateIdentity{}, err
	}
	if metadata != afterMetadata {
		return backupCandidateIdentity{}, fmt.Errorf("legacy local backup candidate changed while snapshotting: %s", absolutePath)
	}
	return backupCandidateIdentity{
		Path:          absolutePath,
		RelativePath:  relativePath,
		Device:        metadata.Device,
		Inode:         metadata.Inode,
		Size:          metadata.Size,
		MTimeUnixNano: metadata.MTimeUnixNano,
		Mode:          metadata.Mode,
		LinkCount:     metadata.LinkCount,
		ContentSHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

type backupCandidateMetadata struct {
	Device        uint64
	Inode         uint64
	Size          int64
	MTimeUnixNano int64
	Mode          uint32
	LinkCount     uint64
}

func backupCandidateMetadataFromInfo(path string, info os.FileInfo) (backupCandidateMetadata, error) {
	if !info.Mode().IsRegular() {
		return backupCandidateMetadata{}, fmt.Errorf("backup candidate is not a regular file: %s", path)
	}
	device, inode, err := fileIdentity(info)
	if err != nil {
		return backupCandidateMetadata{}, fmt.Errorf("inspect backup candidate identity for %s: %w", path, err)
	}
	linkCount, err := backupCandidateLinkCount(path, info)
	if err != nil {
		return backupCandidateMetadata{}, err
	}
	if linkCount != 1 {
		return backupCandidateMetadata{}, fmt.Errorf("legacy local backup candidate must not be hardlinked: %s has %d links", path, linkCount)
	}
	return backupCandidateMetadata{
		Device:        device,
		Inode:         inode,
		Size:          info.Size(),
		MTimeUnixNano: info.ModTime().UnixNano(),
		Mode:          uint32(info.Mode()),
		LinkCount:     linkCount,
	}, nil
}

func secureBackupPathExists(root backupRootIdentity, relativePath string) (bool, error) {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return false, err
	}
	defer rootDirectory.Close()
	parent, base, err := openSecureBackupParent(rootDirectory, relativePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer parent.Close()
	var stat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), base, &stat, unix.AT_SYMLINK_NOFOLLOW); errors.Is(err, unix.ENOENT) {
		return false, nil
	} else if err != nil {
		return false, &os.PathError{Op: "inspect rooted backup candidate", Path: relativePath, Err: err}
	}
	if stat.Mode&unix.S_IFMT == unix.S_IFLNK {
		return false, fmt.Errorf("backup candidate path became a symlink: %s", filepath.Join(root.Path, relativePath))
	}
	return true, nil
}

func secureValidateBackupCandidate(root backupRootIdentity, candidate backupCandidateIdentity) error {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return err
	}
	defer rootDirectory.Close()
	file, err := openSecureBackupCandidate(rootDirectory, candidate.RelativePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return validateOpenedBackupCandidate(file, candidate)
}

func secureConfirmBackupCandidateAbsent(root backupRootIdentity, relativePath string) error {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return err
	}
	defer rootDirectory.Close()
	parent, base, err := openSecureBackupParent(rootDirectory, relativePath)
	if err != nil {
		return err
	}
	defer parent.Close()
	var stat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), base, &stat, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return fmt.Errorf("backup candidate reappeared while confirming deletion: %s", filepath.Join(root.Path, relativePath))
	} else if !errors.Is(err, unix.ENOENT) {
		return &os.PathError{Op: "confirm rooted backup candidate deletion", Path: relativePath, Err: err}
	}
	if err := unix.Fsync(int(parent.Fd())); err != nil {
		return &os.PathError{Op: "sync rooted backup directory", Path: filepath.Dir(filepath.Join(root.Path, relativePath)), Err: err}
	}
	return nil
}

func secureRemoveBackupCandidate(root backupRootIdentity, candidate backupCandidateIdentity) error {
	rootDirectory, err := openSecureBackupRoot(root)
	if err != nil {
		return err
	}
	defer rootDirectory.Close()
	parent, base, err := openSecureBackupParent(rootDirectory, candidate.RelativePath)
	if err != nil {
		return err
	}
	defer parent.Close()
	file, err := openSecureBackupChild(parent, base, candidate.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := validateOpenedBackupCandidate(file, candidate); err != nil {
		return err
	}

	// Recheck that the directory entry still names the descriptor we hashed.
	// unlinkat is rooted at the already-opened parent and therefore cannot
	// traverse a swapped intermediate symlink or escape the committed root.
	var pathStat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), base, &pathStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "revalidate rooted backup candidate", Path: candidate.Path, Err: err}
	}
	if uint64(pathStat.Dev) != candidate.Device || pathStat.Ino != candidate.Inode || pathStat.Mode&unix.S_IFMT != unix.S_IFREG || pathStat.Nlink != 1 {
		return fmt.Errorf("backup candidate directory entry changed before deletion: %s", candidate.Path)
	}
	if err := unix.Unlinkat(int(parent.Fd()), base, 0); err != nil {
		return &os.PathError{Op: "remove rooted backup candidate", Path: candidate.Path, Err: err}
	}
	if err := unix.Fsync(int(parent.Fd())); err != nil {
		return &os.PathError{Op: "sync rooted backup directory", Path: filepath.Dir(candidate.Path), Err: err}
	}
	return nil
}

func validateOpenedBackupCandidate(file *os.File, candidate backupCandidateIdentity) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind backup candidate %s: %w", candidate.Path, err)
	}
	before, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat backup candidate %s: %w", candidate.Path, err)
	}
	metadata, err := backupCandidateMetadataFromInfo(candidate.Path, before)
	if err != nil {
		return err
	}
	if !candidateMetadataMatches(candidate, metadata) {
		return fmt.Errorf("backup candidate identity changed after cutover: %s", candidate.Path)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash backup candidate %s: %w", candidate.Path, err)
	}
	after, err := file.Stat()
	if err != nil {
		return fmt.Errorf("restat backup candidate %s: %w", candidate.Path, err)
	}
	afterMetadata, err := backupCandidateMetadataFromInfo(candidate.Path, after)
	if err != nil {
		return err
	}
	if metadata != afterMetadata || !candidateMetadataMatches(candidate, afterMetadata) {
		return fmt.Errorf("backup candidate changed while validating: %s", candidate.Path)
	}
	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if actualDigest != candidate.ContentSHA256 {
		return fmt.Errorf("backup candidate content digest changed after cutover: %s", candidate.Path)
	}
	return nil
}

func candidateMetadataMatches(candidate backupCandidateIdentity, metadata backupCandidateMetadata) bool {
	return candidate.Device == metadata.Device && candidate.Inode == metadata.Inode &&
		candidate.Size == metadata.Size && candidate.MTimeUnixNano == metadata.MTimeUnixNano &&
		candidate.Mode == metadata.Mode && candidate.LinkCount == metadata.LinkCount
}

func openSecureBackupRoot(expected backupRootIdentity) (*os.File, error) {
	if !filepath.IsAbs(expected.Path) || filepath.Clean(expected.Path) != expected.Path {
		return nil, fmt.Errorf("committed backup root path is not canonical: %s", expected.Path)
	}
	fd, err := unix.Open(string(os.PathSeparator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open filesystem root", Path: string(os.PathSeparator), Err: err}
	}
	current := os.NewFile(uintptr(fd), string(os.PathSeparator))
	if current == nil {
		_ = unix.Close(fd)
		return nil, errors.New("open filesystem root: invalid file descriptor")
	}
	components := strings.Split(strings.TrimPrefix(expected.Path, string(os.PathSeparator)), string(os.PathSeparator))
	if expected.Path == string(os.PathSeparator) {
		components = nil
	}
	for _, component := range components {
		if component == "" || component == "." || component == ".." {
			_ = current.Close()
			return nil, fmt.Errorf("committed backup root path contains an unsafe component: %s", expected.Path)
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			_ = current.Close()
			return nil, &os.PathError{Op: "open backup root without symlinks", Path: expected.Path, Err: openErr}
		}
		_ = current.Close()
		current = os.NewFile(uintptr(nextFD), expected.Path)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, fmt.Errorf("open backup root %s: invalid file descriptor", expected.Path)
		}
	}
	info, err := current.Stat()
	if err != nil {
		_ = current.Close()
		return nil, fmt.Errorf("stat securely opened backup root %s: %w", expected.Path, err)
	}
	device, inode, err := fileIdentity(info)
	if err != nil {
		_ = current.Close()
		return nil, err
	}
	if !info.IsDir() || device != expected.Device || inode != expected.Inode {
		_ = current.Close()
		return nil, fmt.Errorf("legacy local backup root identity changed: expected %s device=%d inode=%d", expected.Path, expected.Device, expected.Inode)
	}
	return current, nil
}

func openSecureBackupCandidate(root *os.File, relativePath string) (*os.File, error) {
	parent, base, err := openSecureBackupParent(root, relativePath)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return openSecureBackupChild(parent, base, relativePath)
}

func openSecureBackupChild(parent *os.File, base, displayPath string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), base, secureBackupOpenFlags, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open backup candidate without symlinks", Path: displayPath, Err: err}
	}
	file := os.NewFile(uintptr(fd), displayPath)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open backup candidate %s: invalid file descriptor", displayPath)
	}
	return file, nil
}

func openSecureBackupParent(root *os.File, relativePath string) (*os.File, string, error) {
	clean := filepath.Clean(relativePath)
	if clean != relativePath || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return nil, "", fmt.Errorf("backup candidate has an unsafe relative path: %s", relativePath)
	}
	components := strings.Split(clean, string(os.PathSeparator))
	base := components[len(components)-1]
	fd, err := unix.Openat(int(root.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", &os.PathError{Op: "duplicate backup root", Path: root.Name(), Err: err}
	}
	current := os.NewFile(uintptr(fd), root.Name())
	if current == nil {
		_ = unix.Close(fd)
		return nil, "", errors.New("duplicate backup root: invalid file descriptor")
	}
	for _, component := range components[:len(components)-1] {
		nextFD, openErr := unix.Openat(int(current.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			_ = current.Close()
			return nil, "", &os.PathError{Op: "open backup parent without symlinks", Path: relativePath, Err: openErr}
		}
		_ = current.Close()
		current = os.NewFile(uintptr(nextFD), relativePath)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, "", fmt.Errorf("open backup parent %s: invalid file descriptor", relativePath)
		}
	}
	return current, base, nil
}
