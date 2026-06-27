package main

import (
	"errors"
	"fmt"

	"github.com/aisphereio/kernel/errorx"
)

const ErrSkillCreateFailed errorx.Code = "AIHUB_SKILL_CREATE_FAILED"

func main() {
	cause := errors.New("pq: connection refused")
	err := errorx.Wrap(cause, ErrSkillCreateFailed,
		errorx.WithMessage("创建技能失败"),
		errorx.WithHTTPStatus(500),
		errorx.WithRetryable(true),
		errorx.WithMetadata("skill_id", "skill_001"),
		errorx.WithRequestID("req_123"),
		errorx.WithTraceID("trace_abc"),
	)

	fmt.Println("code:", errorx.CodeOf(err))
	fmt.Println("status:", errorx.HTTPStatusOf(err))
	fmt.Println("retryable:", errorx.RetryableOf(err))
	fmt.Println("request_id:", errorx.RequestIDOf(err))
}
