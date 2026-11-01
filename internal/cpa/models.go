package cpa

import (
	"encoding/json"
	"strings"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginapi"
)

type rpcAuthModelRequest struct {
	pluginapi.AuthModelRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcModelRouteRequest struct {
	pluginapi.ModelRouteRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

func (p *Plugin) handleModelStatic(raw []byte) (any, error) {
	var req pluginapi.StaticModelRequest
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	models := p.collectCatalogModels()
	return pluginapi.ModelResponse{
		Provider: ProviderID,
		Models:   models,
	}, nil
}

func (p *Plugin) handleModelForAuth(raw []byte) (any, error) {
	var req rpcAuthModelRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	if !isAIStudioProvider(req.AuthProvider) {
		var payload AuthFilePayload
		if len(req.StorageJSON) > 0 {
			_ = json.Unmarshal(req.StorageJSON, &payload)
		}
		if !isAIStudioProvider(payload.Type) && !isAIStudioProvider(payload.Provider) {
			return pluginapi.ModelResponse{}, nil
		}
	}

	var payload AuthFilePayload
	if len(req.StorageJSON) > 0 {
		_ = json.Unmarshal(req.StorageJSON, &payload)
	}
	var authUpdate pluginapi.AuthData
	if state, ok := extractStorageStateFromPayload(req.StorageJSON, payload); ok {
		if rt, err := p.Runtime(); err == nil {
			accID := extractAccountIDFromAuth(req.StorageJSON, req.Metadata, req.Attributes)
			if acc, err := p.upsertAccountInRuntime(p.ctx, rt, accID, payload, state); err == nil && acc.ID != "" {
				if updated, err := p.buildAuthDataForAccount(rt, acc); err == nil {
					if req.AuthID != "" {
						updated.ID = req.AuthID
					}
					authUpdate = updated
				}
			}
		}
	}

	models := p.collectCatalogModelsForAccount(extractAccountIDFromAuth(req.StorageJSON, req.Metadata, req.Attributes))
	return pluginapi.ModelResponse{
		Provider:   ProviderID,
		Models:     models,
		AuthUpdate: authUpdate,
	}, nil
}

func (p *Plugin) handleModelRoute(raw []byte) (any, error) {
	var req rpcModelRouteRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.RequestedModel)
	if model == "" {
		return pluginapi.ModelRouteResponse{Handled: false}, nil
	}

	prefix := strings.TrimSpace(p.Config().ModelPrefix)
	if prefix == "" {
		prefix = "aistudio/"
	}

	lowerModel := strings.ToLower(model)
	for _, pfx := range []string{strings.ToLower(prefix), "aistudio/", "aistudio2api/"} {
		if pfx != "" && strings.HasPrefix(lowerModel, pfx) {
			clean := strings.TrimSpace(model[len(pfx):])
			if clean == "" {
				continue
			}
			return pluginapi.ModelRouteResponse{
				Handled:     true,
				TargetKind:  pluginapi.ModelRouteTargetSelf,
				Target:      PluginID,
				TargetModel: clean,
				Reason:      "routed prefixed AI Studio model to aistudio2api",
			}, nil
		}
	}

	// If route_unprefixed_models is explicitly enabled, or if aistudio2api has active accounts
	// and no competing built-in gemini provider has auth, route known AI Studio models to self.
	routeUnprefixed := boolOrDefault(p.Config().RouteUnprefixedModels, false)
	if !routeUnprefixed {
		hasBuiltinGemini := false
		for _, prov := range req.AvailableProviders {
			pLower := strings.ToLower(strings.TrimSpace(prov))
			if pLower == "gemini" || pLower == "aistudio" || pLower == "vertex" {
				hasBuiltinGemini = true
				break
			}
		}
		if !hasBuiltinGemini && p.hasReadyAccount() && p.isKnownModel(model) {
			routeUnprefixed = true
		}
	}

	if routeUnprefixed && p.isKnownModel(model) {
		return pluginapi.ModelRouteResponse{
			Handled:     true,
			TargetKind:  pluginapi.ModelRouteTargetSelf,
			Target:      PluginID,
			TargetModel: model,
			Reason:      "routed AI Studio model to aistudio2api",
		}, nil
	}

	return pluginapi.ModelRouteResponse{Handled: false}, nil
}

func (p *Plugin) hasReadyAccount() bool {
	rt, err := p.Runtime()
	if err != nil {
		return false
	}
	_, eligible := rt.Pool().EnabledAccounts()
	return eligible > 0
}

func (p *Plugin) isKnownModel(modelID string) bool {
	clean := StripModelPrefix(modelID, p.Config().ModelPrefix)
	clean = strings.TrimPrefix(clean, "models/")
	base, _, err := aistudio.ApplyModelSuffix(clean, aistudio.GenerationConfig{})
	if err == nil && base != "" {
		clean = base
	}
	for _, m := range p.collectRawCatalogModels("") {
		id := strings.TrimPrefix(strings.TrimSpace(m.ID), "models/")
		if strings.EqualFold(id, clean) || strings.EqualFold(id, strings.TrimSuffix(clean, "-search")) {
			return true
		}
	}
	return false
}

func StripModelPrefix(model string, configuredPrefix string) string {
	trimmed := strings.TrimSpace(model)
	lower := strings.ToLower(trimmed)
	for _, pfx := range []string{strings.ToLower(strings.TrimSpace(configuredPrefix)), "aistudio/", "aistudio2api/"} {
		if pfx != "" && strings.HasPrefix(lower, pfx) {
			return strings.TrimSpace(trimmed[len(pfx):])
		}
	}
	return trimmed
}

func (p *Plugin) collectCatalogModels() []pluginapi.ModelInfo {
	return p.collectCatalogModelsForAccount("")
}

func (p *Plugin) collectCatalogModelsForAccount(accountID string) []pluginapi.ModelInfo {
	rawModels := p.collectRawCatalogModels(accountID)
	prefix := strings.TrimSpace(p.Config().ModelPrefix)
	if prefix == "" {
		prefix = "aistudio/"
	}
	searchAliases := boolOrDefault(p.Config().SearchModelAliases, true)

	out := make([]pluginapi.ModelInfo, 0, len(rawModels)*2)
	seen := make(map[string]struct{}, len(rawModels)*2)
	addModel := func(info pluginapi.ModelInfo) {
		key := strings.ToLower(strings.TrimSpace(info.ID))
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, info)
	}

	for _, m := range rawModels {
		info := convertAIStudioModel(m)
		if info.ID == "" {
			continue
		}
		// 1. Register unprefixed model ID (e.g., "gemini-2.5-pro")
		addModel(info)

		// 2. Register prefixed model ID (e.g., "aistudio/gemini-2.5-pro") so it never collides with native providers
		if prefix != "" && !strings.HasPrefix(strings.ToLower(info.ID), strings.ToLower(prefix)) {
			prefixed := info
			prefixed.ID = prefix + info.ID
			prefixed.DisplayName = "[AI Studio] " + info.DisplayName
			addModel(prefixed)
		}

		// 3. Register -search alias for models supporting Google Search grounding
		if searchAliases && m.Capabilities["search"] && !strings.HasSuffix(info.ID, "-search") {
			searchInfo := info
			searchInfo.ID = info.ID + "-search"
			searchInfo.DisplayName = info.DisplayName + " (Search)"
			addModel(searchInfo)
			if prefix != "" {
				prefixedSearch := searchInfo
				prefixedSearch.ID = prefix + searchInfo.ID
				prefixedSearch.DisplayName = "[AI Studio] " + searchInfo.DisplayName
				addModel(prefixedSearch)
			}
		}
	}
	return out
}

func (p *Plugin) collectRawCatalogModels(accountID string) []aistudio.Model {
	rt, err := p.Runtime()
	if err == nil {
		if accountID != "" {
			if acc, ok := rt.Pool().AccountByID(accountID); ok && acc != nil && len(acc.Models) > 0 {
				return acc.Models
			}
		}
		if models, err := rt.Admin.CatalogModels(p.ctx); err == nil && len(models) > 0 {
			return models
		}
		if cached := rt.CachedModels(); len(cached) > 0 {
			return cached
		}
	}
	return defaultFallbackModels()
}

func convertAIStudioModel(m aistudio.Model) pluginapi.ModelInfo {
	id := strings.TrimPrefix(strings.TrimSpace(m.ID), "models/")
	if id == "" {
		return pluginapi.ModelInfo{}
	}
	displayName := strings.TrimSpace(m.Name)
	if displayName == "" {
		displayName = id
	}
	inLimit := m.InputTokenLimit
	if inLimit <= 0 {
		inLimit = 1048576
	}
	outLimit := m.OutputTokenLimit
	if outLimit <= 0 {
		outLimit = 65536
	}

	methods := append([]string(nil), m.Methods...)
	if len(methods) == 0 {
		methods = []string{"generateContent", "streamGenerateContent", "countTokens"}
	}

	var thinking *pluginapi.ThinkingSupport
	if m.ThinkingBudget != nil || len(m.ThinkingLevels) > 0 {
		thinking = &pluginapi.ThinkingSupport{
			DynamicAllowed: true,
			Levels:         append([]string(nil), m.ThinkingLevels...),
		}
		if m.ThinkingBudget != nil {
			if m.ThinkingBudget.Min != nil {
				thinking.Min = int(*m.ThinkingBudget.Min)
			}
			if m.ThinkingBudget.Max != nil {
				thinking.Max = int(*m.ThinkingBudget.Max)
			} else {
				thinking.Max = 32768
			}
			if m.ThinkingBudget.CanDisable != nil {
				thinking.ZeroAllowed = *m.ThinkingBudget.CanDisable
			}
		}
	}

	inputModalities := []string{"text", "image", "audio", "video", "pdf"}
	outputModalities := []string{"text"}
	if m.Capabilities["image_generation"] || strings.Contains(id, "image") || strings.Contains(id, "imagen") {
		outputModalities = append(outputModalities, "image")
	}
	if m.Capabilities["tts"] || strings.Contains(id, "tts") {
		outputModalities = []string{"audio"}
	}
	if m.Capabilities["video_generation"] || strings.Contains(id, "veo") {
		outputModalities = []string{"video"}
	}

	return pluginapi.ModelInfo{
		ID:                         id,
		Object:                     "model",
		OwnedBy:                    ProviderID,
		Type:                       ProviderID,
		DisplayName:                displayName,
		Name:                       "models/" + id,
		Version:                    m.Version,
		Description:                m.Description,
		InputTokenLimit:            inLimit,
		OutputTokenLimit:           outLimit,
		ContextLength:              inLimit,
		MaxCompletionTokens:        outLimit,
		SupportedGenerationMethods: methods,
		SupportedInputModalities:   inputModalities,
		SupportedOutputModalities:  outputModalities,
		Thinking:                   thinking,
		UserDefined:                true,
	}
}

func defaultFallbackModels() []aistudio.Model {
	canDisableTrue := true
	canDisableFalse := false
	minBudget := int64(128)
	maxBudgetPro := int64(32768)
	maxBudgetFlash := int64(24576)

	return []aistudio.Model{
		{
			ID:               "models/gemini-2.5-pro",
			Name:             "Gemini 2.5 Pro",
			Description:      "Google AI Studio Gemini 2.5 Pro with thinking & multimodal reasoning",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  1048576,
			OutputTokenLimit: 65536,
			ThinkingLevels:   []string{"low", "medium", "high"},
			ThinkingBudget:   &aistudio.ThinkingBudget{Min: &minBudget, Max: &maxBudgetPro, CanDisable: &canDisableFalse},
			Capabilities:     map[string]bool{"thinking": true, "search": true, "code_execution": true, "url_context": true, "function_calling": true, "structured_outputs": true},
		},
		{
			ID:               "models/gemini-2.5-flash",
			Name:             "Gemini 2.5 Flash",
			Description:      "Google AI Studio Gemini 2.5 Flash hybrid reasoning model",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  1048576,
			OutputTokenLimit: 65536,
			ThinkingLevels:   []string{"minimal", "low", "medium", "high"},
			ThinkingBudget:   &aistudio.ThinkingBudget{Min: &minBudget, Max: &maxBudgetFlash, CanDisable: &canDisableTrue},
			Capabilities:     map[string]bool{"thinking": true, "search": true, "code_execution": true, "url_context": true, "function_calling": true, "structured_outputs": true},
		},
		{
			ID:               "models/gemini-2.5-flash-lite",
			Name:             "Gemini 2.5 Flash-Lite",
			Description:      "Google AI Studio Gemini 2.5 Flash-Lite high-efficiency model",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  1048576,
			OutputTokenLimit: 65536,
			ThinkingLevels:   []string{"minimal", "low", "medium", "high"},
			ThinkingBudget:   &aistudio.ThinkingBudget{Min: &minBudget, Max: &maxBudgetFlash, CanDisable: &canDisableTrue},
			Capabilities:     map[string]bool{"thinking": true, "search": true, "code_execution": true, "url_context": true, "function_calling": true, "structured_outputs": true},
		},
		{
			ID:               "models/gemini-3-pro-preview",
			Name:             "Gemini 3 Pro Preview",
			Description:      "Google AI Studio Gemini 3 Pro Preview",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  1048576,
			OutputTokenLimit: 65536,
			ThinkingLevels:   []string{"low", "medium", "high"},
			ThinkingBudget:   &aistudio.ThinkingBudget{Min: &minBudget, Max: &maxBudgetPro, CanDisable: &canDisableFalse},
			Capabilities:     map[string]bool{"thinking": true, "search": true, "code_execution": true, "url_context": true, "function_calling": true, "structured_outputs": true},
		},
		{
			ID:               "models/gemini-3-flash-preview",
			Name:             "Gemini 3 Flash Preview",
			Description:      "Google AI Studio Gemini 3 Flash Preview",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  1048576,
			OutputTokenLimit: 65536,
			ThinkingLevels:   []string{"minimal", "low", "medium", "high"},
			ThinkingBudget:   &aistudio.ThinkingBudget{Min: &minBudget, Max: &maxBudgetFlash, CanDisable: &canDisableTrue},
			Capabilities:     map[string]bool{"thinking": true, "search": true, "code_execution": true, "url_context": true, "function_calling": true, "structured_outputs": true},
		},
		{
			ID:               "models/gemini-2.5-flash-image-preview",
			Name:             "Gemini 2.5 Flash Image Preview (Nano Banana)",
			Description:      "Google AI Studio native multimodal image generation & editing",
			Methods:          []string{"generateContent", "streamGenerateContent", "countTokens"},
			InputTokenLimit:  32768,
			OutputTokenLimit: 8192,
			Capabilities:     map[string]bool{"image_generation": true},
		},
		{
			ID:               "models/gemini-2.5-pro-preview-tts",
			Name:             "Gemini 2.5 Pro Preview TTS",
			Description:      "Google AI Studio Gemini 2.5 Pro Text-to-Speech",
			Methods:          []string{"generateContent"},
			InputTokenLimit:  8192,
			OutputTokenLimit: 16384,
			Capabilities:     map[string]bool{"tts": true},
		},
		{
			ID:               "models/gemini-2.5-flash-preview-tts",
			Name:             "Gemini 2.5 Flash Preview TTS",
			Description:      "Google AI Studio Gemini 2.5 Flash Text-to-Speech",
			Methods:          []string{"generateContent"},
			InputTokenLimit:  8192,
			OutputTokenLimit: 16384,
			Capabilities:     map[string]bool{"tts": true},
		},
		{
			ID:               "models/imagen-3.0-generate-002",
			Name:             "Imagen 3",
			Description:      "Google AI Studio Imagen 3 image generation",
			Methods:          []string{"predict"},
			InputTokenLimit:  480,
			OutputTokenLimit: 4,
			Capabilities:     map[string]bool{"image_generation": true},
		},
		{
			ID:               "models/veo-2.0-generate-001",
			Name:             "Veo 2",
			Description:      "Google AI Studio Veo 2 video generation",
			Methods:          []string{"predictLongRunning"},
			InputTokenLimit:  480,
			OutputTokenLimit: 1,
			Capabilities:     map[string]bool{"video_generation": true},
		},
		{
			ID:               "models/gemini-embedding-001",
			Name:             "Gemini Embedding 001",
			Description:      "Google AI Studio text embedding model",
			Methods:          []string{"embedContent", "batchEmbedContents"},
			InputTokenLimit:  8192,
			OutputTokenLimit: 3072,
		},
	}
}
