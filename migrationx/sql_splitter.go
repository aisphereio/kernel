package migrationx

import "strings"

type sqlSplitState struct {
	singleQuoted      bool
	doubleQuoted      bool
	backtickQuoted    bool
	dollarQuote       string
	lineComment       bool
	blockCommentDepth int
}

func (s sqlSplitState) normal() bool {
	return !s.singleQuoted &&
		!s.doubleQuoted &&
		!s.backtickQuoted &&
		s.dollarQuote == "" &&
		!s.lineComment &&
		s.blockCommentDepth == 0
}

// splitSQLStatements separates SQL only at statement terminators that are
// outside quoted strings, quoted identifiers, PostgreSQL dollar-quoted bodies,
// backtick-quoted identifiers, and comments. Goose StatementBegin/StatementEnd
// blocks remain supported for dialect-specific constructs that should be sent
// to the driver as one unit.
func splitSQLStatements(sqlText string) []string {
	var out []string
	var current strings.Builder
	var state sqlSplitState
	inStatementBlock := false
	hasSQL := false

	flush := func() {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" && hasSQL {
			out = append(out, stmt)
		}
		current.Reset()
		hasSQL = false
	}

	for _, line := range strings.SplitAfter(sqlText, "\n") {
		trimmed := strings.TrimSpace(line)
		directive := strings.ToLower(trimmed)

		if inStatementBlock {
			if directive == "-- +goose statementend" {
				inStatementBlock = false
				flush()
				state = sqlSplitState{}
				continue
			}
			current.WriteString(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
				hasSQL = true
			}
			continue
		}

		if state.normal() && directive == "-- +goose statementbegin" {
			flush()
			inStatementBlock = true
			continue
		}

		scanSQLChunk(line, &state, &current, &hasSQL, flush)
	}
	flush()
	return out
}

func scanSQLChunk(
	sqlText string,
	state *sqlSplitState,
	current *strings.Builder,
	hasSQL *bool,
	flush func(),
) {
	for i := 0; i < len(sqlText); {
		if state.lineComment {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '\n' {
				state.lineComment = false
			}
			i++
			continue
		}

		if state.blockCommentDepth > 0 {
			switch {
			case strings.HasPrefix(sqlText[i:], "/*"):
				current.WriteString("/*")
				state.blockCommentDepth++
				i += 2
			case strings.HasPrefix(sqlText[i:], "*/"):
				current.WriteString("*/")
				state.blockCommentDepth--
				i += 2
			default:
				current.WriteByte(sqlText[i])
				i++
			}
			continue
		}

		if state.dollarQuote != "" {
			if strings.HasPrefix(sqlText[i:], state.dollarQuote) {
				current.WriteString(state.dollarQuote)
				i += len(state.dollarQuote)
				state.dollarQuote = ""
			} else {
				current.WriteByte(sqlText[i])
				i++
			}
			continue
		}

		if state.singleQuoted {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '\'' {
				if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				state.singleQuoted = false
			}
			i++
			continue
		}

		if state.doubleQuoted {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '"' {
				if i+1 < len(sqlText) && sqlText[i+1] == '"' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				state.doubleQuoted = false
			}
			i++
			continue
		}

		if state.backtickQuoted {
			current.WriteByte(sqlText[i])
			if sqlText[i] == '`' {
				if i+1 < len(sqlText) && sqlText[i+1] == '`' {
					current.WriteByte(sqlText[i+1])
					i += 2
					continue
				}
				state.backtickQuoted = false
			}
			i++
			continue
		}

		switch {
		case strings.HasPrefix(sqlText[i:], "--"):
			current.WriteString("--")
			state.lineComment = true
			i += 2
		case strings.HasPrefix(sqlText[i:], "/*"):
			current.WriteString("/*")
			state.blockCommentDepth = 1
			i += 2
		case sqlText[i] == '\'':
			current.WriteByte(sqlText[i])
			state.singleQuoted = true
			*hasSQL = true
			i++
		case sqlText[i] == '"':
			current.WriteByte(sqlText[i])
			state.doubleQuoted = true
			*hasSQL = true
			i++
		case sqlText[i] == '`':
			current.WriteByte(sqlText[i])
			state.backtickQuoted = true
			*hasSQL = true
			i++
		case sqlText[i] == '$':
			delimiter, ok := dollarQuoteDelimiter(sqlText, i)
			if ok {
				current.WriteString(delimiter)
				state.dollarQuote = delimiter
				*hasSQL = true
				i += len(delimiter)
				continue
			}
			current.WriteByte(sqlText[i])
			*hasSQL = true
			i++
		case sqlText[i] == ';':
			flush()
			i++
		default:
			current.WriteByte(sqlText[i])
			if !isSQLSpace(sqlText[i]) {
				*hasSQL = true
			}
			i++
		}
	}
}

func dollarQuoteDelimiter(sqlText string, start int) (string, bool) {
	if start > 0 && isSQLIdentifierByte(sqlText[start-1]) {
		return "", false
	}
	end := start + 1
	if end >= len(sqlText) {
		return "", false
	}
	if sqlText[end] == '$' {
		return "$$", true
	}
	if !isDollarTagStart(sqlText[end]) {
		return "", false
	}
	end++
	for end < len(sqlText) && isDollarTagContinue(sqlText[end]) {
		end++
	}
	if end >= len(sqlText) || sqlText[end] != '$' {
		return "", false
	}
	return sqlText[start : end+1], true
}

func isDollarTagStart(b byte) bool {
	return b == '_' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func isDollarTagContinue(b byte) bool {
	return isDollarTagStart(b) || b >= '0' && b <= '9'
}

func isSQLIdentifierByte(b byte) bool {
	return isDollarTagContinue(b) || b == '$' || b >= 0x80
}

func isSQLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}
