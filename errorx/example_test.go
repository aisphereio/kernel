package errorx_test

import (
	"errors"
	"fmt"

	"github.com/aisphereio/kernel/errorx"
)

func ExampleNotFound() {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")

	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.HTTPStatusOf(err))
	fmt.Println(errorx.RetryableOf(err))

	// Output:
	// AIHUB_SKILL_NOT_FOUND
	// 404
	// false
}

func ExampleWrap() {
	cause := errors.New("pq: connection refused")
	err := errorx.Wrap(cause, "AIHUB_SKILL_CREATE_FAILED",
		errorx.WithMessage("创建技能失败"),
		errorx.WithRetryable(true),
	)

	fmt.Println(errorx.CodeOf(err))
	fmt.Println(errorx.MessageOf(err))
	fmt.Println(errorx.RetryableOf(err))
	fmt.Println(errors.Is(err, cause))

	// Output:
	// AIHUB_SKILL_CREATE_FAILED
	// 创建技能失败
	// true
	// true
}

func ExampleFields() {
	err := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除权限",
		errorx.WithRequestID("req_123"),
	)
	fields := errorx.Fields(err)

	fmt.Println(fields["error_code"])
	fmt.Println(fields["http_status"])
	fmt.Println(fields["request_id"])

	// Output:
	// AIHUB_SKILL_DELETE_DENIED
	// 403
	// req_123
}
