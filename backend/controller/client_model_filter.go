package controller

import (
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

type clientModelFamily string

const (
	clientModelFamilyAll    clientModelFamily = "all"
	clientModelFamilyClaude clientModelFamily = "claude"
	clientModelFamilyOpenAI clientModelFamily = "openai"
)

func detectClientModelFamily(c *gin.Context, declaredClient string) clientModelFamily {
	if strings.HasPrefix(c.Request.URL.Path, "/anthropic/") {
		return clientModelFamilyClaude
	}
	fingerprint := strings.ToLower(strings.Join([]string{
		declaredClient,
		c.GetHeader("Originator"),
		c.GetHeader("X-Client-Name"),
		c.GetHeader("X-Client"),
		c.GetHeader("X-App"),
		c.Request.UserAgent(),
	}, " "))
	if strings.Contains(fingerprint, "claude") || strings.Contains(fingerprint, "anthropic") {
		return clientModelFamilyClaude
	}
	if strings.Contains(fingerprint, "codex") || strings.Contains(fingerprint, "chatgpt") || strings.Contains(fingerprint, "openai") {
		return clientModelFamilyOpenAI
	}
	return clientModelFamilyAll
}

func filterModelsForClient(models []string, family clientModelFamily) []string {
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		normalized := strings.ToLower(strings.TrimSpace(modelName))
		baseName := normalized
		if slash := strings.LastIndex(baseName, "/"); slash >= 0 {
			baseName = baseName[slash+1:]
		}

		include := family == clientModelFamilyAll
		switch family {
		case clientModelFamilyClaude:
			include = strings.Contains(normalized, "anthropic/") ||
				strings.Contains(baseName, "claude") || strings.Contains(baseName, "sonnet") ||
				strings.Contains(baseName, "opus") || strings.Contains(baseName, "haiku")
		case clientModelFamilyOpenAI:
			include = strings.Contains(baseName, "gpt") ||
				strings.Contains(baseName, "chatgpt") || strings.Contains(baseName, "codex") ||
				strings.HasPrefix(baseName, "o1") || strings.HasPrefix(baseName, "o3") ||
				strings.HasPrefix(baseName, "o4") || strings.HasPrefix(baseName, "text-embedding-") ||
				strings.HasPrefix(baseName, "text-moderation-") || strings.HasPrefix(baseName, "omni-moderation-") ||
				strings.HasPrefix(baseName, "dall-e-") || strings.HasPrefix(baseName, "whisper-") ||
				strings.HasPrefix(baseName, "tts-") || strings.HasPrefix(baseName, "sora-")
		}
		if include {
			filtered = append(filtered, modelName)
		}
	}
	sort.Strings(filtered)
	return filtered
}
