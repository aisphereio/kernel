// Package errorx defines Aisphere Kernel's standard error semantics.
//
// The package is intentionally dependency-light and uses only the Go standard
// library. It does not log, write HTTP responses, emit metrics or record audit
// events. Other Kernel modules consume its stable contract through inspect
// helpers such as CodeOf, HTTPStatusOf, RetryableOf, Fields and MetricsLabels.
package errorx

// SupportPackageIsVersion1 is used by generated proto errorx code to assert
// compile-time compatibility with this errorx package.
const SupportPackageIsVersion1 = true
