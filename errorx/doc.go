// Package errorx defines Aisphere Kernel's standard error semantics.
//
// errorx is the ONLY runtime/business error package in Kernel. It replaces the
// old Kratos-derived Kernel errors package.
//
// # Design principle
//
// errorx only DEFINES error semantics. It does NOT log, write HTTP responses,
// emit metrics, record audit events, or call gRPC. Other Kernel modules
// (logx, httpx, grpcx, auditx, metricsx, workerx) CONSUME errorx through
// stable inspect helpers such as [CodeOf], [HTTPStatusOf], [RetryableOf],
// [Fields], and [MetricsLabels].
//
// errorx depends only on the Go standard library.
//
// # 30-second quickstart
//
//	func GetSkill(ctx context.Context, id string) (*Skill, error) {
//	    skill, err := repo.Find(ctx, id)
//	    if errors.Is(err, sql.ErrNoRows) {
//	        return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
//	            errorx.WithMetadata("skill_id", id),
//	            errorx.WithPublicMetadata("resource", "skill"),
//	        )
//	    }
//	    if err != nil {
//	        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
//	            errorx.WithMessage("查询技能失败"),
//	            errorx.WithRetryable(true),
//	        )
//	    }
//	    return skill, nil
//	}
//
// # Constructors
//
// Use semantic constructors instead of manually setting HTTP/gRPC status:
//
//	errorx.BadRequest("REQUEST_VALIDATE_FAILED", "请求参数校验失败")     // 400
//	errorx.Unauthorized("AUTH_TOKEN_MISSING", "缺少授权令牌")            // 401
//	errorx.Forbidden("IAM_PERMISSION_DENIED", "权限被拒绝")             // 403
//	errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")               // 404
//	errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在")         // 409
//	errorx.RequestTimeout("REQUEST_TIMEOUT", "请求超时")                // 408
//	errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "请求过于频繁")       // 429
//	errorx.ClientClosed("CLIENT_CLOSED", "客户端断开连接")              // 499
//	errorx.Internal("AIHUB_INTERNAL_ERROR", "服务器内部错误")           // 500
//	errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型上游不可用")  // 503
//	errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型上游超时")            // 504
//
// # Options
//
// Append metadata, cause, retryable, etc. via options:
//
//	errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
//	    errorx.WithMessage("查询技能失败"),
//	    errorx.WithRetryable(true),
//	    errorx.WithMetadata("skill_id", id),          // internal use
//	    errorx.WithPublicMetadata("resource", "skill"), // safe for clients
//	    errorx.WithRequestID("req_123"),
//	    errorx.WithTraceID("trace_abc"),
//	    errorx.WithStack(),
//	)
//
// # Inspect helpers (consumed by logx/httpx/grpcx/auditx/metricsx/workerx)
//
// Do NOT type-assert on *Error. Use inspect helpers which are nil-safe and
// recognize wrapped third-party errors via errors.As:
//
//	errorx.CodeOf(err)            // AIHUB_SKILL_NOT_FOUND (nil → OK, unknown → INTERNAL_ERROR)
//	errorx.HTTPStatusOf(err)      // 404 (nil → 200)
//	errorx.GRPCCodeOf(err)        // NotFound
//	errorx.RetryableOf(err)       // false
//	errorx.Fields(err)            // map[string]any for logx
//	errorx.MetricsLabels(err)     // map[string]string low-cardinality labels for metricsx
//	errorx.IsNotFound(err)        // true (predicate shortcut)
//
// # Error code naming
//
// Format: {DOMAIN}_{RESOURCE}_{REASON}, uppercase snake_case.
//
//	AIHUB_SKILL_NOT_FOUND       // good
//	IAM_TOKEN_INVALID           // good
//	MODEL_UPSTREAM_TIMEOUT      // good
//	skillNotFound               // bad: camelCase
//	SKILL_123_NOT_FOUND         // bad: dynamic ID in code
//
// Dynamic values MUST go into metadata, never into the error code:
//
//	errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
//	    errorx.WithMetadata("skill_id", skillID),  // dynamic value here
//	)
//
// # Metadata safety
//
// errorx distinguishes three metadata layers:
//
//	Metadata()         — internal use, returned as defensive copy
//	SafeMetadataOf()   — redacted copy (password/token/secret → [REDACTED])
//	PublicMetadata()   — explicitly safe for HTTP/gRPC responses
//
// NEVER put tokens, passwords, cookies, or private keys into PublicMetadata.
//
// # Forbidden in business code
//
// Handler/service/repository code MUST NOT use:
//
//	errors.New("skill not found")
//	fmt.Errorf("create failed: %w", err)
//	panic(err)
//
// Use errorx constructors or [Wrap] instead.
//
// # Debugging with %+v
//
// [Error.Error] returns a safe message. Use fmt.Printf("%+v", err) for full
// debug output including code, http_status, grpc_code, retryable, category,
// severity, request_id, trace_id, metadata, and cause.
//
// # Further reading
//
// See errorx/README.md for the single-source-of-truth user guide, and
// docs/ai/errorx.md for the AI coding recipe.
package errorx
