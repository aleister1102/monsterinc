package differ

// DiffResult interface for models that represent diff results
type DiffResult interface {
	GetDiffs() interface{}
	IsIdentical() bool
}
