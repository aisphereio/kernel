package dbx

import (
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// buildOnConflictClause constructs an ON CONFLICT DO UPDATE clause for
// GORM's Clauses() API. Works for both postgres (ON CONFLICT) and mysql
// (ON DUPLICATE KEY UPDATE) because GORM translates the same clause to
// the appropriate SQL based on the dialector.
//
// Returns clause.Expression so dbx.go can pass it to gormDB.Clauses()
// without importing gorm/clause directly.
func buildOnConflictClause(updateColumns []string) clause.Expression {
	return clause.OnConflict{
		UpdateAll: false,
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}
}

// gormExpr wraps gorm.Expr so dbx.go does not need to import gorm directly
// in the increment function (keeps the dbx.go file smaller and easier to
// audit for security-relevant code paths).
func gormExpr(sql string, args ...any) any {
	return gorm.Expr(sql, args...)
}

// sliceLen returns the length of a slice via reflection. Used by paginate
// to determine hasMore without knowing dest's concrete type.
func sliceLen(dest any) int {
	v := reflect.ValueOf(dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return 0
	}
	return v.Len()
}

// sliceTruncate truncates dest to the first n elements. Used by paginate
// to remove the size+1th element used to detect hasMore.
func sliceTruncate(dest any, n int) {
	v := reflect.ValueOf(dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return
	}
	if v.Len() <= n {
		return
	}
	// If dest is *[]T, v is the slice and is addressable via the pointer.
	if ptr := reflect.ValueOf(dest); ptr.Kind() == reflect.Ptr && ptr.Elem().CanSet() {
		ptr.Elem().Set(v.Slice(0, n))
	}
}
