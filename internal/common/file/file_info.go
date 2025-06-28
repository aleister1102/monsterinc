package file

import (
	"context"
	"io/fs"
	"time"
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

// FileWriteOptions configures file writing behavior
type FileWriteOptions struct {
	CreateDirs  bool            // Whether to create parent directories
	Permissions fs.FileMode     // File permissions
	Timeout     time.Duration   // Write timeout
	Context     context.Context // Context for cancellation
}

// DefaultFileReadOptions returns default file reading options
func DefaultFileReadOptions() FileReadOptions {
	return FileReadOptions{
		MaxSize:    50 * 1024 * 1024, // 50MB default
		BufferSize: 64 * 1024,        // 64KB buffer
		LineBased:  false,
		TrimLines:  true,
		SkipEmpty:  true,
		Timeout:    30 * time.Second,
		Context:    context.Background(),
	}
}

// DefaultFileWriteOptions returns default file writing options
func DefaultFileWriteOptions() FileWriteOptions {
	return FileWriteOptions{
		CreateDirs:  true,
		Permissions: 0644,
		Timeout:     30 * time.Second,
		Context:     context.Background(),
	}
}
