// Package main is a reference handler template showing correct errorx usage.
//
// AI coding tools should use this file as a starting point when generating
// new business handlers. Copy this file, rename the package and types, and
// fill in the business logic.
//
// Key points illustrated:
//   1. Error codes declared as errorx.Code constants at module top
//   2. Constructor chosen by semantic (NotFound / BadRequest / Forbidden / ...)
//   3. Cause preserved with errorx.Wrap or WithCause
//   4. Internal metadata via WithMetadata, public metadata via WithPublicMetadata
//   5. NO use of errors.New / fmt.Errorf / panic in business code
//   6. NO use of log.Printf / fmt.Println — use logx.FromContext(ctx)
//
// See docs/ai/errorx.md for the full AI coding recipe.
package main

import (
	"context"
	"errors"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

// ============================================================================
// 1. Domain error codes — declare once per module, uppercase snake_case
// ============================================================================

const (
	ErrSkillNotFound      errorx.Code = "AIHUB_SKILL_NOT_FOUND"
	ErrSkillNameRequired  errorx.Code = "AIHUB_SKILL_NAME_REQUIRED"
	ErrSkillNameTooLong   errorx.Code = "AIHUB_SKILL_NAME_TOO_LONG"
	ErrSkillCreateDenied  errorx.Code = "AIHUB_SKILL_CREATE_DENIED"
	ErrSkillAlreadyExists errorx.Code = "AIHUB_SKILL_ALREADY_EXISTS"
	ErrSkillQueryFailed   errorx.Code = "AIHUB_SKILL_QUERY_FAILED"
	ErrSkillCreateFailed  errorx.Code = "AIHUB_SKILL_CREATE_FAILED"
)

// ============================================================================
// 2. Domain model (DO — domain object, no proto, no storage tags)
// ============================================================================

type Skill struct {
	ID          string
	Name        string
	DisplayName string
	OwnerID     string
}

// ============================================================================
// 3. Repository interface (declared in biz, implemented in data)
// ============================================================================

type SkillRepo interface {
	Find(ctx context.Context, id string) (*Skill, error)
	Create(ctx context.Context, skill *Skill) error
}

// ============================================================================
// 4. Service (usecase) — business rules + authz + repo calls
// ============================================================================

type SkillService struct {
	repo SkillRepo
}

func NewSkillService(repo SkillRepo) *SkillService {
	return &SkillService{repo: repo}
}

func (s *SkillService) Get(ctx context.Context, id string) (*Skill, error) {
	skill, err := s.repo.Find(ctx, id)
	if errors.Is(err, errRecordNotFound) {
		return nil, errorx.NotFound(ErrSkillNotFound, "技能不存在",
			errorx.WithCause(err),
			errorx.WithMetadata("skill_id", id),
			errorx.WithPublicMetadata("resource", "skill"),
		)
	}
	if err != nil {
		return nil, errorx.Wrap(err, ErrSkillQueryFailed,
			errorx.WithMessage("查询技能失败"),
			errorx.WithRetryable(true),
			errorx.WithMetadata("skill_id", id),
		)
	}
	return skill, nil
}

func (s *SkillService) Create(ctx context.Context, req *CreateSkillRequest) (*Skill, error) {
	// --- 4.1 Validation ---
	if req.Name == "" {
		return nil, errorx.BadRequest(ErrSkillNameRequired, "技能名称不能为空")
	}
	if len(req.Name) > 128 {
		return nil, errorx.BadRequest(ErrSkillNameTooLong, "技能名称不能超过 128 字符",
			errorx.WithPublicMetadata("field", "name"),
			errorx.WithPublicMetadata("max", 128),
		)
	}

	// --- 4.2 Authz (use kernel access.Guard in real code) ---
	// if err := rt.Access.Require(ctx, access.Check{
	//     Resource: "aihub:skill:*",
	//     Action:   "skill.create",
	// }); err != nil {
	//     return nil, errorx.Forbidden(ErrSkillCreateDenied, "没有创建技能的权限",
	//         errorx.WithCause(err),
	//     )
	// }

	// --- 4.3 Business logic: check duplicate ---
	existing, err := s.repo.Find(ctx, req.Name)
	if err != nil && !errors.Is(err, errRecordNotFound) {
		return nil, errorx.Wrap(err, ErrSkillQueryFailed,
			errorx.WithMessage("检查技能是否存在失败"),
		)
	}
	if existing != nil {
		return nil, errorx.Conflict(ErrSkillAlreadyExists, "技能名已存在",
			errorx.WithMetadata("existing_skill_id", existing.ID),
			errorx.WithPublicMetadata("name", req.Name),
		)
	}

	// --- 4.4 Persist ---
	skill := &Skill{
		ID:          req.Name,
		Name:        req.Name,
		DisplayName: req.DisplayName,
	}
	if err := s.repo.Create(ctx, skill); err != nil {
		return nil, errorx.Wrap(err, ErrSkillCreateFailed,
			errorx.WithMessage("创建技能失败"),
			errorx.WithRetryable(true),
			errorx.WithMetadata("skill_id", skill.ID),
		)
	}

	// --- 4.5 Log success (use logx, never log.Printf) ---
	logx.FromContext(ctx).Info("skill created",
		logx.String("skill_id", skill.ID),
		logx.String("name", skill.Name),
	)

	return skill, nil
}

// ============================================================================
// 5. Request DTO
// ============================================================================

type CreateSkillRequest struct {
	Name        string
	DisplayName string
}

// ============================================================================
// 6. Sentinel error for "record not found" from repo (replace with gorm.ErrRecordNotFound in real code)
// ============================================================================

var errRecordNotFound = errors.New("record not found")
