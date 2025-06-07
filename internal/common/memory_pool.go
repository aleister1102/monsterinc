package common

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of byte buffers to reduce allocations
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool with the specified initial capacity
func NewBufferPool(initialCapacity int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, initialCapacity))
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf != nil {
		buf.Reset()
		bp.pool.Put(buf)
	}
}

// SlicePool manages a pool of byte slices to reduce allocations
type SlicePool struct {
	pool sync.Pool
}

// NewSlicePool creates a new slice pool with the specified initial capacity
func NewSlicePool(initialCapacity int) *SlicePool {
	return &SlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				slice := make([]byte, 0, initialCapacity)
				return &slice
			},
		},
	}
}

// Get retrieves a slice from the pool
func (sp *SlicePool) Get() []byte {
	return *(sp.pool.Get().(*[]byte))
}

// Put returns a slice to the pool after resetting it
func (sp *SlicePool) Put(slice []byte) {
	if slice != nil {
		slice = slice[:0] // Reset length but keep capacity
		sp.pool.Put(&slice)
	}
}

// StringSlicePool manages a pool of string slices
type StringSlicePool struct {
	pool sync.Pool
}

// NewStringSlicePool creates a new string slice pool
func NewStringSlicePool(initialCapacity int) *StringSlicePool {
	return &StringSlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				slice := make([]string, 0, initialCapacity)
				return &slice
			},
		},
	}
}

// Get retrieves a string slice from the pool
func (ssp *StringSlicePool) Get() []string {
	return *(ssp.pool.Get().(*[]string))
}

// Put returns a string slice to the pool after resetting it
func (ssp *StringSlicePool) Put(slice []string) {
	if slice != nil {
		slice = slice[:0] // Reset length but keep capacity
		ssp.pool.Put(&slice)
	}
}

// Global pools for common use cases
var (
	// Default buffer pool for 64KB buffers
	DefaultBufferPool = NewBufferPool(64 * 1024)

	// Default slice pool for 32KB slices
	DefaultSlicePool = NewSlicePool(32 * 1024)

	// Default string slice pool for 100 strings
	DefaultStringSlicePool = NewStringSlicePool(100)
)
