package common

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// FileInfo contains metadata about a file
type FileInfo struct {
	Path        string      // Full file path
	Name        string      // File name only
	Size        int64       // File size in bytes
	IsDir       bool        // Whether it's a directory
	ModTime     time.Time   // Last modification time
	Permissions fs.FileMode // File permissions
}

// FileReadOptions configures file reading behavior
type FileReadOptions struct {
	MaxSize    int64           // Maximum file size to read (0 = no limit)
	BufferSize int             // Buffer size for reading
	LineBased  bool            // Whether to read line by line
	TrimLines  bool            // Whether to trim whitespace from lines
	SkipEmpty  bool            // Whether to skip empty lines
	Timeout    time.Duration   // Read timeout
	Context    context.Context // Context for cancellation
}

// DefaultFileReadOptions returns default file reading options
func DefaultFileReadOptions() FileReadOptions {
	return FileReadOptions{
		MaxSize:    10 * 1024 * 1024, // 10MB default
		BufferSize: 64 * 1024,        // 64KB buffer
		LineBased:  false,
		TrimLines:  true,
		SkipEmpty:  true,
		Timeout:    30 * time.Second,
		Context:    context.Background(),
	}
}

// FileWriteOptions configures file writing behavior
type FileWriteOptions struct {
	CreateDirs  bool            // Whether to create parent directories
	Append      bool            // Whether to append to existing file
	Backup      bool            // Whether to backup existing file
	Permissions fs.FileMode     // File permissions (default 0644)
	DirPerms    fs.FileMode     // Directory permissions (default 0755)
	Sync        bool            // Whether to sync to disk immediately
	Timeout     time.Duration   // Write timeout
	Context     context.Context // Context for cancellation
}

// DefaultFileWriteOptions returns default file writing options
func DefaultFileWriteOptions() FileWriteOptions {
	return FileWriteOptions{
		CreateDirs:  true,
		Append:      false,
		Backup:      false,
		Permissions: 0644,
		DirPerms:    0755,
		Sync:        false,
		Timeout:     30 * time.Second,
		Context:     context.Background(),
	}
}

// FileManager provides high-level file operations with standardized error handling and logging
type FileManager struct {
	logger zerolog.Logger
}

// NewFileManager creates a new FileManager instance
func NewFileManager(logger zerolog.Logger) *FileManager {
	return &FileManager{
		logger: logger.With().Str("component", "FileManager").Logger(),
	}
}

// FileExists checks if a file or directory exists
func (fm *FileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	exists := !os.IsNotExist(err)
	fm.logger.Debug().Str("path", path).Bool("exists", exists).Msg("File existence check")
	return exists
}

// GetFileInfo returns information about a file
func (fm *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, WrapError(ErrNotFound, fmt.Sprintf("file not found: %s", path))
		}
		return nil, WrapError(err, fmt.Sprintf("failed to get file info for: %s", path))
	}

	info := &FileInfo{
		Path:        path,
		Name:        stat.Name(),
		Size:        stat.Size(),
		IsDir:       stat.IsDir(),
		ModTime:     stat.ModTime(),
		Permissions: stat.Mode(),
	}

	fm.logger.Debug().
		Str("path", path).
		Int64("size", info.Size).
		Bool("is_dir", info.IsDir).
		Time("mod_time", info.ModTime).
		Msg("Retrieved file info")

	return info, nil
}

// validateFileForReading validates a file path and options before reading
func (fm *FileManager) validateFileForReading(path string, opts FileReadOptions) (*FileInfo, error) {
	// Check if file exists and validate
	info, err := fm.GetFileInfo(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir {
		return nil, NewValidationError("path", path, "is a directory, not a file")
	}

	if opts.MaxSize > 0 && info.Size > opts.MaxSize {
		return nil, NewValidationError("file_size", info.Size, fmt.Sprintf("exceeds maximum size of %d bytes", opts.MaxSize))
	}

	return info, nil
}

// setupContextWithTimeout sets up context with timeout if specified
func (fm *FileManager) setupContextWithTimeout(opts FileReadOptions) (context.Context, context.CancelFunc) {
	ctx := opts.Context
	if opts.Timeout > 0 {
		return context.WithTimeout(ctx, opts.Timeout)
	}
	return ctx, nil
}

// createBufferedReader creates a buffered reader if buffer size is specified
func (fm *FileManager) createBufferedReader(file *os.File, bufferSize int) io.Reader {
	var reader io.Reader = file
	if bufferSize > 0 {
		reader = bufio.NewReaderSize(file, bufferSize)
	}
	return reader
}

// performFileRead performs the actual file reading operation with context support
func (fm *FileManager) performFileRead(path string, reader io.Reader, opts FileReadOptions, ctx context.Context) ([]byte, error) {
	done := make(chan struct{})
	var content []byte
	var readErr error

	go func() {
		defer close(done)
		if opts.LineBased {
			content, readErr = fm.readFileLines(reader, opts)
		} else {
			content, readErr = fm.readFileContent(reader, opts.MaxSize)
		}
	}()

	select {
	case <-ctx.Done():
		fm.logger.Warn().Str("path", path).Msg("File read cancelled due to context timeout")
		return nil, WrapError(ctx.Err(), "file read operation cancelled")
	case <-done:
		if readErr != nil {
			return nil, WrapError(readErr, fmt.Sprintf("failed to read file content: %s", path))
		}
	}

	return content, nil
}

// ReadFile reads a file with the given options
func (fm *FileManager) ReadFile(path string, opts FileReadOptions) ([]byte, error) {
	fm.logger.Debug().Str("path", path).Msg("Starting file read operation")

	// Validate file and options
	_, err := fm.validateFileForReading(path, opts)
	if err != nil {
		return nil, err
	}

	// Set up context with timeout if specified
	ctx, cancel := fm.setupContextWithTimeout(opts)
	if cancel != nil {
		defer cancel()
	}

	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, WrapError(err, fmt.Sprintf("failed to open file: %s", path))
	}
	defer file.Close()

	// Create buffered reader if buffer size is specified
	reader := fm.createBufferedReader(file, opts.BufferSize)

	// Perform file read with context support
	content, err := fm.performFileRead(path, reader, opts, ctx)
	if err != nil {
		return nil, err
	}

	fm.logger.Debug().
		Str("path", path).
		Int("bytes_read", len(content)).
		Msg("File read completed successfully")

	return content, nil
}

// readFileContent reads file content into bytes
func (fm *FileManager) readFileContent(reader io.Reader, maxSize int64) ([]byte, error) {
	if maxSize > 0 {
		reader = io.LimitReader(reader, maxSize)
	}
	return io.ReadAll(reader)
}

// readFileLines reads file content line by line
func (fm *FileManager) readFileLines(reader io.Reader, opts FileReadOptions) ([]byte, error) {
	scanner := bufio.NewScanner(reader)
	var lines []string

	for scanner.Scan() {
		line := scanner.Text()

		if opts.TrimLines {
			line = strings.TrimSpace(line)
		}

		if opts.SkipEmpty && line == "" {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// prepareDirectoriesForWrite creates parent directories if needed
func (fm *FileManager) prepareDirectoriesForWrite(path string, opts FileWriteOptions) error {
	if opts.CreateDirs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, opts.DirPerms); err != nil {
			return WrapError(err, fmt.Sprintf("failed to create directory: %s", dir))
		}
	}
	return nil
}

// createBackupIfNeeded creates a backup of existing file if requested
func (fm *FileManager) createBackupIfNeeded(path string, opts FileWriteOptions) error {
	if opts.Backup && fm.FileExists(path) {
		backupPath := path + ".bak." + time.Now().Format("20060102-150405")
		if err := fm.copyFile(path, backupPath); err != nil {
			fm.logger.Warn().Err(err).Str("path", path).Str("backup", backupPath).Msg("Failed to create backup file")
		} else {
			fm.logger.Info().Str("path", path).Str("backup", backupPath).Msg("Created backup file")
		}
	}
	return nil
}

// determineFileFlags determines the appropriate file flags for opening
func (fm *FileManager) determineFileFlags(opts FileWriteOptions) int {
	flags := os.O_WRONLY | os.O_CREATE
	if opts.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	return flags
}

// performFileWrite performs the actual file writing operation
func (fm *FileManager) performFileWrite(path string, data []byte, opts FileWriteOptions) error {
	flags := fm.determineFileFlags(opts)

	file, err := os.OpenFile(path, flags, opts.Permissions)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to open file for writing: %s", path))
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to write data to file: %s", path))
	}

	if opts.Sync {
		if err := file.Sync(); err != nil {
			return WrapError(err, fmt.Sprintf("failed to sync file to disk: %s", path))
		}
	}

	return nil
}

// WriteFile writes data to a file with the given options
func (fm *FileManager) WriteFile(path string, data []byte, opts FileWriteOptions) error {
	fm.logger.Debug().
		Str("path", path).
		Int("data_size", len(data)).
		Bool("append", opts.Append).
		Msg("Starting file write operation")

	// Set up context with timeout if specified
	ctx := opts.Context
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Create parent directories if needed
	if err := fm.prepareDirectoriesForWrite(path, opts); err != nil {
		return err
	}

	// Backup existing file if requested
	if err := fm.createBackupIfNeeded(path, opts); err != nil {
		return err
	}

	// Write with context cancellation support
	done := make(chan error, 1)
	go func() {
		done <- fm.performFileWrite(path, data, opts)
	}()

	select {
	case <-ctx.Done():
		fm.logger.Warn().Str("path", path).Msg("File write cancelled due to context timeout")
		return WrapError(ctx.Err(), "file write operation cancelled")
	case err := <-done:
		if err != nil {
			return err
		}
	}

	fm.logger.Debug().
		Str("path", path).
		Int("bytes_written", len(data)).
		Msg("File write completed successfully")

	return nil
}

// copyFile copies a file from src to dst
func (fm *FileManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to open source file: %s", src))
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to create destination file: %s", dst))
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to copy data from %s to %s", src, dst))
	}

	return destFile.Sync()
}

// ReadLines reads a file and returns it as a slice of lines
func (fm *FileManager) ReadLines(path string, opts FileReadOptions) ([]string, error) {
	opts.LineBased = true
	content, err := fm.ReadFile(path, opts)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []string{}, nil
	}

	lines := strings.Split(string(content), "\n")

	// Remove empty last line if present (common with text files)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

// WriteLines writes a slice of lines to a file
func (fm *FileManager) WriteLines(path string, lines []string, opts FileWriteOptions) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Add final newline
	}
	return fm.WriteFile(path, []byte(content), opts)
}

// AppendLine appends a single line to a file
func (fm *FileManager) AppendLine(path string, line string) error {
	opts := DefaultFileWriteOptions()
	opts.Append = true
	content := line
	if !strings.HasSuffix(line, "\n") {
		content += "\n"
	}
	return fm.WriteFile(path, []byte(content), opts)
}

// MoveFile moves a file from src to dst
func (fm *FileManager) MoveFile(src, dst string) error {
	fm.logger.Debug().Str("src", src).Str("dst", dst).Msg("Moving file")

	// Try atomic rename first (works if src and dst are on same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		fm.logger.Debug().Str("src", src).Str("dst", dst).Msg("File moved successfully via rename")
		return nil
	}

	// Fall back to copy and delete
	if err := fm.copyFile(src, dst); err != nil {
		return WrapError(err, fmt.Sprintf("failed to copy file from %s to %s", src, dst))
	}

	if err := os.Remove(src); err != nil {
		fm.logger.Warn().Err(err).Str("src", src).Msg("Failed to remove source file after copy")
		return WrapError(err, fmt.Sprintf("failed to remove source file: %s", src))
	}

	fm.logger.Debug().Str("src", src).Str("dst", dst).Msg("File moved successfully via copy and delete")
	return nil
}

// DeleteFile deletes a file or directory
func (fm *FileManager) DeleteFile(path string) error {
	fm.logger.Debug().Str("path", path).Msg("Deleting file")

	info, err := fm.GetFileInfo(path)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			fm.logger.Debug().Str("path", path).Msg("File does not exist, nothing to delete")
			return nil // Not an error if file doesn't exist
		}
		return err
	}

	if info.IsDir {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to delete: %s", path))
	}

	fm.logger.Debug().Str("path", path).Bool("was_dir", info.IsDir).Msg("File deleted successfully")
	return nil
}

// ListDirectory lists files and directories in a directory
func (fm *FileManager) ListDirectory(path string, recursive bool) ([]FileInfo, error) {
	fm.logger.Debug().Str("path", path).Bool("recursive", recursive).Msg("Listing directory")

	var files []FileInfo

	if recursive {
		err := filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				fm.logger.Warn().Err(err).Str("path", filePath).Msg("Error walking directory")
				return nil // Continue walking despite error
			}

			info, err := d.Info()
			if err != nil {
				fm.logger.Warn().Err(err).Str("path", filePath).Msg("Error getting file info")
				return nil // Continue walking despite error
			}

			files = append(files, FileInfo{
				Path:        filePath,
				Name:        info.Name(),
				Size:        info.Size(),
				IsDir:       info.IsDir(),
				ModTime:     info.ModTime(),
				Permissions: info.Mode(),
			})

			return nil
		})

		if err != nil {
			return nil, WrapError(err, fmt.Sprintf("failed to walk directory: %s", path))
		}
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, WrapError(err, fmt.Sprintf("failed to read directory: %s", path))
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				fm.logger.Warn().Err(err).Str("name", entry.Name()).Msg("Error getting file info for directory entry")
				continue
			}

			filePath := filepath.Join(path, entry.Name())
			files = append(files, FileInfo{
				Path:        filePath,
				Name:        info.Name(),
				Size:        info.Size(),
				IsDir:       info.IsDir(),
				ModTime:     info.ModTime(),
				Permissions: info.Mode(),
			})
		}
	}

	fm.logger.Debug().
		Str("path", path).
		Int("file_count", len(files)).
		Bool("recursive", recursive).
		Msg("Directory listing completed")

	return files, nil
}

// EnsureDirectory creates a directory and its parents if they don't exist
func (fm *FileManager) EnsureDirectory(path string, perms fs.FileMode) error {
	if fm.FileExists(path) {
		info, err := fm.GetFileInfo(path)
		if err != nil {
			return err
		}
		if !info.IsDir {
			return NewValidationError("path", path, "exists but is not a directory")
		}
		return nil // Directory already exists
	}

	fm.logger.Debug().Str("path", path).Str("perms", perms.String()).Msg("Creating directory")

	err := os.MkdirAll(path, perms)
	if err != nil {
		return WrapError(err, fmt.Sprintf("failed to create directory: %s", path))
	}

	fm.logger.Debug().Str("path", path).Msg("Directory created successfully")
	return nil
}

// GetFileSize returns the size of a file in bytes
func (fm *FileManager) GetFileSize(path string) (int64, error) {
	info, err := fm.GetFileInfo(path)
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

// IsDirectory checks if the path is a directory
func (fm *FileManager) IsDirectory(path string) (bool, error) {
	info, err := fm.GetFileInfo(path)
	if err != nil {
		return false, err
	}
	return info.IsDir, nil
}

// GetDirectorySize calculates the total size of a directory and its contents
func (fm *FileManager) GetDirectorySize(path string) (int64, error) {
	var totalSize int64

	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, WrapError(err, fmt.Sprintf("failed to calculate directory size: %s", path))
	}

	return totalSize, nil
}

// SafeFileWriter provides atomic file writing capabilities
type SafeFileWriter struct {
	targetPath string
	tempPath   string
	file       *os.File
	fm         *FileManager
}

// NewSafeFileWriter creates a new safe file writer that writes to a temporary file first
func (fm *FileManager) NewSafeFileWriter(targetPath string, perms fs.FileMode) (*SafeFileWriter, error) {
	tempPath := targetPath + ".tmp." + time.Now().Format("20060102-150405")

	// Ensure parent directory exists
	if err := fm.EnsureDirectory(filepath.Dir(targetPath), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perms)
	if err != nil {
		return nil, WrapError(err, fmt.Sprintf("failed to create temporary file: %s", tempPath))
	}

	return &SafeFileWriter{
		targetPath: targetPath,
		tempPath:   tempPath,
		file:       file,
		fm:         fm,
	}, nil
}

// Write writes data to the temporary file
func (sfw *SafeFileWriter) Write(data []byte) (int, error) {
	return sfw.file.Write(data)
}

// Commit closes the temporary file and atomically moves it to the target location
func (sfw *SafeFileWriter) Commit() error {
	if err := sfw.file.Sync(); err != nil {
		return WrapError(err, "failed to sync temporary file")
	}

	if err := sfw.file.Close(); err != nil {
		return WrapError(err, "failed to close temporary file")
	}

	if err := os.Rename(sfw.tempPath, sfw.targetPath); err != nil {
		return WrapError(err, fmt.Sprintf("failed to move temporary file to target: %s", sfw.targetPath))
	}

	sfw.fm.logger.Debug().
		Str("target", sfw.targetPath).
		Str("temp", sfw.tempPath).
		Msg("Safe file write completed successfully")

	return nil
}

// Abort cancels the write operation and cleans up the temporary file
func (sfw *SafeFileWriter) Abort() error {
	var errs []error

	if sfw.file != nil {
		if err := sfw.file.Close(); err != nil {
			errs = append(errs, WrapError(err, "failed to close temporary file during abort"))
		}
	}

	if err := os.Remove(sfw.tempPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, WrapError(err, "failed to remove temporary file during abort"))
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}

	return nil
}

// Global convenience functions that use a default FileManager

var defaultFileManager = NewFileManager(zerolog.Nop())

// QuickReadFile reads a file with default options
func QuickReadFile(path string) ([]byte, error) {
	return defaultFileManager.ReadFile(path, DefaultFileReadOptions())
}

// QuickWriteFile writes a file with default options
func QuickWriteFile(path string, data []byte) error {
	return defaultFileManager.WriteFile(path, data, DefaultFileWriteOptions())
}

// QuickReadLines reads a file as lines with default options
func QuickReadLines(path string) ([]string, error) {
	return defaultFileManager.ReadLines(path, DefaultFileReadOptions())
}

// QuickWriteLines writes lines to a file with default options
func QuickWriteLines(path string, lines []string) error {
	return defaultFileManager.WriteLines(path, lines, DefaultFileWriteOptions())
}

// QuickFileExists checks if a file exists
func QuickFileExists(path string) bool {
	return defaultFileManager.FileExists(path)
}

// QuickEnsureDir creates a directory with default permissions
func QuickEnsureDir(path string) error {
	return defaultFileManager.EnsureDirectory(path, 0755)
}
