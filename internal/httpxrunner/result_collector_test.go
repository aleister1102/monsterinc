package telescope_test

import (
	"sync"
	"testing"

	"github.com/aleister1102/go-telescope"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestResultCollector_CollectAndGet(t *testing.T) {
	logger := zerolog.Nop()
	mapper := telescope.NewProbeResultMapper(logger)
	collector := telescope.NewResultCollector(logger, mapper, "http://example.com")

	// This is a simplified test because the mapping logic is not implemented yet.
	// We'll simulate adding results by calling the Collect method.
	// In a real scenario, the httpx engine would call this.

	collector.Collect(runner.Result{Input: "http://example.com"})
	collector.Collect(runner.Result{Input: "http://test.com"})

	// Since the simplified Collect method doesn't actually add to the slice,
	// we expect the result slice to be empty.
	// A full implementation would require mocking the mapping and checking the slice content.
	results := collector.GetResults()
	assert.NotEmpty(t, results, "Expected results to not be empty")
	assert.Len(t, results, 2, "Expected 2 results")

	// TODO: Expand this test when the mapping from runner.Result to TelescopeResult is implemented.
}

func TestResultCollector_Concurrency(t *testing.T) {
	logger := zerolog.Nop()
	mapper := telescope.NewProbeResultMapper(logger)
	collector := telescope.NewResultCollector(logger, mapper, "")

	var wg sync.WaitGroup
	numRoutines := 100

	// Again, this is simplified. We're just calling the method concurrently.
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collector.Collect(runner.Result{})
		}()
	}

	wg.Wait()

	results := collector.GetResults()
	assert.Len(t, results, numRoutines, "Expected results to be empty with simplified Collect")
	// In a full implementation, you'd assert len(results) == numRoutines
}

func TestResultCollector_EdgeCases(t *testing.T) {
	logger := zerolog.Nop()
	mapper := telescope.NewProbeResultMapper(logger)

	t.Run("Add nil result", func(t *testing.T) {
		collector := telescope.NewResultCollector(logger, mapper, "")
		collector.Collect(runner.Result{}) // This result has no error, so a TelescopeResult is created
		assert.Equal(t, 1, len(collector.GetResults()), "Should add results")
	})

	t.Run("Empty collector", func(t *testing.T) {
		collector := telescope.NewResultCollector(logger, mapper, "")
		assert.Equal(t, 0, len(collector.GetResults()), "Count should be 0 for new collector")
		assert.NotNil(t, collector.GetResults(), "GetResults should return a non-nil slice")
		assert.Len(t, collector.GetResults(), 0, "GetResults should return an empty slice")
	})

	t.Run("Initial capacity zero", func(t *testing.T) {
		collector := telescope.NewResultCollector(logger, mapper, "http://example.com")
		assert.Equal(t, 0, len(collector.GetResults()))

		collector.Collect(runner.Result{Input: "http://example.com"})
		assert.Equal(t, 1, len(collector.GetResults()))
		assert.Len(t, collector.GetResults(), 1)
	})
}
