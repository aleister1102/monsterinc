package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBufferPool(t *testing.T) {
	tests := []struct {
		name            string
		initialCapacity int
	}{
		{
			name:            "small capacity",
			initialCapacity: 10,
		},
		{
			name:            "medium capacity",
			initialCapacity: 1024,
		},
		{
			name:            "large capacity",
			initialCapacity: 65536,
		},
		{
			name:            "zero capacity",
			initialCapacity: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := NewBufferPool(tt.initialCapacity)
			assert.NotNil(t, bp)

			// Test that we can get a buffer
			buf := bp.Get()
			assert.NotNil(t, buf)
			assert.IsType(t, &bytes.Buffer{}, buf)

			// Buffer should be empty after getting from pool
			assert.Equal(t, 0, buf.Len())
		})
	}
}

func TestBufferPool_GetAndPut(t *testing.T) {
	bp := NewBufferPool(1024)

	// Get a buffer and write some data
	buf1 := bp.Get()
	assert.NotNil(t, buf1)

	testData := "test data for buffer pool"
	buf1.WriteString(testData)
	assert.Equal(t, len(testData), buf1.Len())

	// Put the buffer back
	bp.Put(buf1)

	// Get another buffer - should be reset
	buf2 := bp.Get()
	assert.NotNil(t, buf2)
	assert.Equal(t, 0, buf2.Len()) // Should be reset

	// Write to the second buffer
	buf2.WriteString("another test")
	assert.Equal(t, 12, buf2.Len())

	// Put it back
	bp.Put(buf2)
}

func TestBufferPool_MultipleOperations(t *testing.T) {
	bp := NewBufferPool(512)
	buffers := make([]*bytes.Buffer, 5)

	// Get multiple buffers
	for i := 0; i < 5; i++ {
		buffers[i] = bp.Get()
		assert.NotNil(t, buffers[i])
		assert.Equal(t, 0, buffers[i].Len())

		// Write unique data to each buffer
		buffers[i].WriteString("data " + string(rune('A'+i)))
	}

	// Verify each buffer has its own data
	for i := 0; i < 5; i++ {
		expected := "data " + string(rune('A'+i))
		assert.Equal(t, expected, buffers[i].String())
	}

	// Put all buffers back
	for i := 0; i < 5; i++ {
		bp.Put(buffers[i])
	}

	// Get buffers again - should be reset
	for i := 0; i < 5; i++ {
		buf := bp.Get()
		assert.NotNil(t, buf)
		assert.Equal(t, 0, buf.Len())
		bp.Put(buf)
	}
}

func TestNewSlicePool(t *testing.T) {
	tests := []struct {
		name            string
		initialCapacity int
	}{
		{
			name:            "small capacity",
			initialCapacity: 10,
		},
		{
			name:            "medium capacity",
			initialCapacity: 1024,
		},
		{
			name:            "large capacity",
			initialCapacity: 65536,
		},
		{
			name:            "zero capacity",
			initialCapacity: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := NewSlicePool(tt.initialCapacity)
			assert.NotNil(t, sp)

			// Test that we can get a slice
			slice := sp.Get()
			assert.NotNil(t, slice)
			assert.IsType(t, []byte{}, slice)

			// Slice should be empty after getting from pool
			assert.Equal(t, 0, len(slice))

			// But should have some capacity
			if tt.initialCapacity > 0 {
				assert.True(t, cap(slice) >= tt.initialCapacity)
			}
		})
	}
}

func TestSlicePool_GetAndPut(t *testing.T) {
	sp := NewSlicePool(1024)

	// Get a slice and append some data
	slice1 := sp.Get()
	assert.NotNil(t, slice1)
	assert.Equal(t, 0, len(slice1))

	testData := []byte("test data for slice pool")
	slice1 = append(slice1, testData...)
	assert.Equal(t, len(testData), len(slice1))

	// Put the slice back
	sp.Put(slice1)

	// Get another slice - should be reset
	slice2 := sp.Get()
	assert.NotNil(t, slice2)
	assert.Equal(t, 0, len(slice2)) // Should be reset

	// Append to the second slice
	slice2 = append(slice2, []byte("another test")...)
	assert.Equal(t, 12, len(slice2))

	// Put it back
	sp.Put(slice2)
}

func TestSlicePool_MultipleOperations(t *testing.T) {
	sp := NewSlicePool(512)
	slices := make([][]byte, 5)

	// Get multiple slices
	for i := 0; i < 5; i++ {
		slices[i] = sp.Get()
		assert.NotNil(t, slices[i])
		assert.Equal(t, 0, len(slices[i]))

		// Append unique data to each slice
		data := []byte("data " + string(rune('A'+i)))
		slices[i] = append(slices[i], data...)
	}

	// Verify each slice has its own data
	for i := 0; i < 5; i++ {
		expected := "data " + string(rune('A'+i))
		assert.Equal(t, expected, string(slices[i]))
	}

	// Put all slices back
	for i := 0; i < 5; i++ {
		sp.Put(slices[i])
	}

	// Get slices again - should be reset
	for i := 0; i < 5; i++ {
		slice := sp.Get()
		assert.NotNil(t, slice)
		assert.Equal(t, 0, len(slice))
		sp.Put(slice)
	}
}

func TestNewStringSlicePool(t *testing.T) {
	tests := []struct {
		name            string
		initialCapacity int
	}{
		{
			name:            "small capacity",
			initialCapacity: 10,
		},
		{
			name:            "medium capacity",
			initialCapacity: 100,
		},
		{
			name:            "large capacity",
			initialCapacity: 1000,
		},
		{
			name:            "zero capacity",
			initialCapacity: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssp := NewStringSlicePool(tt.initialCapacity)
			assert.NotNil(t, ssp)

			// Test that we can get a string slice
			slice := ssp.Get()
			assert.NotNil(t, slice)
			assert.IsType(t, []string{}, slice)

			// Slice should be empty after getting from pool
			assert.Equal(t, 0, len(slice))

			// But should have some capacity
			if tt.initialCapacity > 0 {
				assert.True(t, cap(slice) >= tt.initialCapacity)
			}
		})
	}
}

func TestStringSlicePool_GetAndPut(t *testing.T) {
	ssp := NewStringSlicePool(100)

	// Get a slice and append some data
	slice1 := ssp.Get()
	assert.NotNil(t, slice1)
	assert.Equal(t, 0, len(slice1))

	testData := []string{"test", "data", "for", "string", "slice", "pool"}
	slice1 = append(slice1, testData...)
	assert.Equal(t, len(testData), len(slice1))

	// Put the slice back
	ssp.Put(slice1)

	// Get another slice - should be reset
	slice2 := ssp.Get()
	assert.NotNil(t, slice2)
	assert.Equal(t, 0, len(slice2)) // Should be reset

	// Append to the second slice
	slice2 = append(slice2, "another", "test")
	assert.Equal(t, 2, len(slice2))

	// Put it back
	ssp.Put(slice2)
}

func TestStringSlicePool_MultipleOperations(t *testing.T) {
	ssp := NewStringSlicePool(50)
	slices := make([][]string, 5)

	// Get multiple slices
	for i := 0; i < 5; i++ {
		slices[i] = ssp.Get()
		assert.NotNil(t, slices[i])
		assert.Equal(t, 0, len(slices[i]))

		// Append unique data to each slice
		data := "data" + string(rune('A'+i))
		slices[i] = append(slices[i], data)
	}

	// Verify each slice has its own data
	for i := 0; i < 5; i++ {
		expected := "data" + string(rune('A'+i))
		assert.Equal(t, 1, len(slices[i]))
		assert.Equal(t, expected, slices[i][0])
	}

	// Put all slices back
	for i := 0; i < 5; i++ {
		ssp.Put(slices[i])
	}

	// Get slices again - should be reset
	for i := 0; i < 5; i++ {
		slice := ssp.Get()
		assert.NotNil(t, slice)
		assert.Equal(t, 0, len(slice))
		ssp.Put(slice)
	}
}

func TestMemoryPoolsConcurrency(t *testing.T) {
	// Test concurrent access to buffer pool
	bp := NewBufferPool(1024)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				buf := bp.Get()
				buf.WriteString("goroutine " + string(rune('0'+id)))
				bp.Put(buf)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the pool is still functional
	buf := bp.Get()
	assert.NotNil(t, buf)
	assert.Equal(t, 0, buf.Len())
	bp.Put(buf)
}
