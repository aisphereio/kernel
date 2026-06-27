package errorx

import "strings"

const Redacted = "[REDACTED]"

var sensitiveMetadataTokens = []string{
	"password",
	"passwd",
	"pwd",
	"token",
	"secret",
	"authorization",
	"cookie",
	"credential",
	"private_key",
	"apikey",
	"api_key",
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeMap(dst map[string]any, src map[string]any) map[string]any {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for k, v := range src {
		if strings.TrimSpace(k) == "" {
			continue
		}
		dst[k] = v
	}
	return dst
}

func redactMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if isSensitiveMetadataKey(k) {
			dst[k] = Redacted
			continue
		}
		dst[k] = v
	}
	return dst
}

func isSensitiveMetadataKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, token := range sensitiveMetadataTokens {
		if strings.Contains(k, token) {
			return true
		}
	}
	return false
}
