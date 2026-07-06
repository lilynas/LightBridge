package routes

import (
	"net/http"

	"github.com/WilliamWang1721/LightBridge/internal/config"
	"github.com/WilliamWang1721/LightBridge/internal/handler"
	"github.com/WilliamWang1721/LightBridge/internal/server/middleware"
	"github.com/WilliamWang1721/LightBridge/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterGatewayRoutes 注册 API 网关路由（Claude/OpenAI/Gemini 兼容）
func RegisterGatewayRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	privacyFilterService *service.PrivacyFilterService,
	cfg *config.Config,
) {
	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()
	// 响应侧隐私脱敏中间件（内部按需短路；未启用时无开销）。
	privacyResp := middleware.PrivacyFilterResponseWriter(privacyFilterService)

	// 未分组 Key 拦截中间件（按协议格式区分错误响应）
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	requireGroupGoogle := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)

	// API网关（Claude API兼容）
	gateway := r.Group("/v1")
	gateway.Use(bodyLimit)
	gateway.Use(clientRequestID)
	gateway.Use(opsErrorLogger)
	gateway.Use(endpointNorm)
	gateway.Use(gin.HandlerFunc(apiKeyAuth))
	gateway.Use(requireGroupAnthropic)
	gateway.Use(privacyResp)
	{
		gateway.POST("/messages", func(c *gin.Context) {
			switch getGroupPlatform(c) {
			case service.PlatformGrok:
				grokUnsupported(c, "/v1/messages")
			case service.PlatformOpenAI:
				h.OpenAIGateway.Messages(c)
			default:
				h.Gateway.Messages(c)
			}
		})
		gateway.POST("/messages/count_tokens", func(c *gin.Context) {
			switch getGroupPlatform(c) {
			case service.PlatformOpenAI, service.PlatformGrok:
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
					"type":    "not_found_error",
					"message": "Token counting is not supported for this platform",
				}})
			default:
				h.Gateway.CountTokens(c)
			}
		})
		gateway.GET("/models", h.Gateway.Models)
		gateway.GET("/usage", h.Gateway.Usage)
		gateway.POST("/responses", openAICompatibleResponsesHandler(h))
		gateway.POST("/responses/*subpath", openAICompatibleResponsesHandler(h))
		gateway.GET("/responses", func(c *gin.Context) {
			if getGroupPlatform(c) == service.PlatformGrok {
				grokUnsupported(c, "/v1/responses websocket")
				return
			}
			h.OpenAIGateway.ResponsesWebSocket(c)
		})
		gateway.POST("/chat/completions", chatCompletionsHandler(h, "/v1/chat/completions"))
		gateway.POST("/embeddings", openAIOnlyHandler(h.OpenAIGateway.Embeddings, "/v1/embeddings"))
		gateway.POST("/images/generations", openAIOnlyHandler(h.OpenAIGateway.Images, "/v1/images/generations"))
		gateway.POST("/images/edits", openAIOnlyHandler(h.OpenAIGateway.Images, "/v1/images/edits"))
	}

	// Gemini 原生 API 兼容层（Gemini SDK/CLI 直连）
	gemini := r.Group("/v1beta")
	gemini.Use(bodyLimit)
	gemini.Use(clientRequestID)
	gemini.Use(opsErrorLogger)
	gemini.Use(endpointNorm)
	gemini.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	gemini.Use(requireGroupGoogle)
	gemini.Use(privacyResp)
	{
		gemini.GET("/models", h.Gateway.GeminiV1BetaListModels)
		gemini.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		// Gin treats ":" as a param marker, but Gemini uses "{model}:{action}" in the same segment.
		gemini.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

	// OpenAI Responses API（不带v1前缀的别名）
	responsesHandler := openAICompatibleResponsesHandler(h)
	r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, responsesHandler)
	r.POST("/responses/*subpath", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, responsesHandler)
	r.GET("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			grokUnsupported(c, "/responses websocket")
			return
		}
		h.OpenAIGateway.ResponsesWebSocket(c)
	})
	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp)
	{
		codexDirect.POST("/responses", responsesHandler)
		codexDirect.POST("/responses/*subpath", responsesHandler)
		codexDirect.GET("/responses", func(c *gin.Context) {
			if getGroupPlatform(c) == service.PlatformGrok {
				grokUnsupported(c, "/backend-api/codex/responses websocket")
				return
			}
			h.OpenAIGateway.ResponsesWebSocket(c)
		})
	}
	// OpenAI Chat Completions API（不带v1前缀的别名）
	r.POST("/chat/completions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, chatCompletionsHandler(h, "/chat/completions"))
	r.POST("/embeddings", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, openAIOnlyHandler(h.OpenAIGateway.Embeddings, "/embeddings"))
	r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, openAIOnlyHandler(h.OpenAIGateway.Images, "/images/generations"))
	r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, privacyResp, openAIOnlyHandler(h.OpenAIGateway.Images, "/images/edits"))

	// Antigravity 模型列表
	r.GET("/antigravity/models", gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.Gateway.AntigravityModels)

	// Antigravity 专用路由（仅使用 antigravity 账户，不混合调度）
	antigravityV1 := r.Group("/antigravity/v1")
	antigravityV1.Use(bodyLimit)
	antigravityV1.Use(clientRequestID)
	antigravityV1.Use(opsErrorLogger)
	antigravityV1.Use(endpointNorm)
	antigravityV1.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1.Use(gin.HandlerFunc(apiKeyAuth))
	antigravityV1.Use(requireGroupAnthropic)
	antigravityV1.Use(privacyResp)
	{
		antigravityV1.POST("/messages", h.Gateway.Messages)
		antigravityV1.POST("/messages/count_tokens", h.Gateway.CountTokens)
		antigravityV1.GET("/models", h.Gateway.AntigravityModels)
		antigravityV1.GET("/usage", h.Gateway.Usage)
	}

	antigravityV1Beta := r.Group("/antigravity/v1beta")
	antigravityV1Beta.Use(bodyLimit)
	antigravityV1Beta.Use(clientRequestID)
	antigravityV1Beta.Use(opsErrorLogger)
	antigravityV1Beta.Use(endpointNorm)
	antigravityV1Beta.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1Beta.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	antigravityV1Beta.Use(requireGroupGoogle)
	antigravityV1Beta.Use(privacyResp)
	{
		antigravityV1Beta.GET("/models", h.Gateway.GeminiV1BetaListModels)
		antigravityV1Beta.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		antigravityV1Beta.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

}

func getGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}

func grokUnsupported(c *gin.Context, endpoint string) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
		"type":    "not_found_error",
		"message": endpoint + " is not supported for Grok groups",
	}})
}

func openAICompatibleResponsesHandler(h *handler.Handlers) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI, service.PlatformGrok:
			h.OpenAIGateway.Responses(c)
		default:
			h.Gateway.Responses(c)
		}
	}
}

func chatCompletionsHandler(h *handler.Handlers, endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok:
			grokUnsupported(c, endpoint)
		case service.PlatformOpenAI:
			h.OpenAIGateway.ChatCompletions(c)
		default:
			h.Gateway.ChatCompletions(c)
		}
	}
}

func openAIOnlyHandler(next gin.HandlerFunc, endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI:
			next(c)
		case service.PlatformGrok:
			grokUnsupported(c, endpoint)
		default:
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"type":    "not_found_error",
				"message": endpoint + " is only supported for OpenAI groups",
			}})
		}
	}
}

func shouldUseOpenAIHandler(c *gin.Context) bool {
	return isOpenAIInboundEndpoint(handler.GetInboundEndpoint(c))
}

func shouldUseGeminiHandler(c *gin.Context) bool {
	return handler.RequiredProtocolForInboundEndpoint(handler.GetInboundEndpoint(c)) == service.CustomProtocolGemini
}

func isOpenAIInboundEndpoint(inbound string) bool {
	switch handler.RequiredProtocolForInboundEndpoint(inbound) {
	case service.CustomProtocolOpenAIResponses,
		service.CustomProtocolOpenAIChatCompletions,
		service.CustomProtocolOpenAIEmbeddings:
		return true
	default:
		return inbound == handler.EndpointImagesGenerations || inbound == handler.EndpointImagesEdits
	}
}
