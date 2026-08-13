package dbx

// NormalizeGORMError converts an error returned by the GORM escape hatch into
// Kernel's canonical dbx error model.
//
// Higher-level Kernel packages such as dbrepo may use GORM to build typed,
// constrained queries, but they must not leak gorm/driver-specific errors to
// business services. This helper keeps the normalization boundary inside dbx.
func NormalizeGORMError(err error) error {
	return wrapDriverErr(err)
}
