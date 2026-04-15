package handlers

import (
	"encoding/json"
	"net/http"
	"path"
	"strconv"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type MaterialHandler struct {
	app *appstate.App
}

// 创建素材Handler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewMaterialHandler(app *appstate.App) *MaterialHandler {
	return &MaterialHandler{app: app}
}

// 处理素材Roots接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) Roots(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))

	items, err := h.app.Store.ListMaterialRootsByOwner(r.Context(), user.ID, deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material roots")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

// 处理素材列表接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	rootName := strings.TrimSpace(r.URL.Query().Get("root"))
	relativePath := normalizeMaterialPath(r.URL.Query().Get("path"))
	if deviceID == "" || rootName == "" {
		render.Error(w, http.StatusBadRequest, "deviceId and root are required")
		return
	}

	root, err := h.app.Store.GetMaterialRootByOwner(r.Context(), user.ID, deviceID, rootName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material root")
		return
	}
	if root == nil {
		render.Error(w, http.StatusNotFound, "Material root not found")
		return
	}

	items, err := h.app.Store.ListMaterialEntriesByOwner(r.Context(), user.ID, deviceID, rootName, relativePath)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material directory")
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"deviceId": deviceID,
		"root":     rootName,
		"rootPath": root.RootPath,
		"path":     relativePath,
		"entries":  items,
	})
}

// 处理素材文件接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) File(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	rootName := strings.TrimSpace(r.URL.Query().Get("root"))
	relativePath := normalizeMaterialPath(r.URL.Query().Get("path"))
	if deviceID == "" || rootName == "" || relativePath == "" {
		render.Error(w, http.StatusBadRequest, "deviceId, root, and path are required")
		return
	}

	item, err := h.app.Store.GetMaterialEntryByOwner(r.Context(), user.ID, deviceID, rootName, relativePath)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material file")
		return
	}
	if item == nil {
		render.Error(w, http.StatusNotFound, "Material file not found")
		return
	}
	render.JSON(w, http.StatusOK, item)
}

// 处理素材工作区接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	rootName := strings.TrimSpace(r.URL.Query().Get("root"))
	relativePath := normalizeMaterialPath(r.URL.Query().Get("path"))
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	if deviceID == "" || rootName == "" || relativePath == "" {
		render.Error(w, http.StatusBadRequest, "deviceId, root, and path are required")
		return
	}

	root, err := h.app.Store.GetMaterialRootByOwner(r.Context(), user.ID, deviceID, rootName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material root")
		return
	}
	if root == nil {
		render.Error(w, http.StatusNotFound, "Material root not found")
		return
	}

	entry, err := h.app.Store.GetMaterialEntryByOwner(r.Context(), user.ID, deviceID, rootName, relativePath)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load material entry")
		return
	}
	if entry == nil {
		render.Error(w, http.StatusNotFound, "Material entry not found")
		return
	}

	subtree := false
	switch scope {
	case "", "auto":
		subtree = entry.Kind == "directory"
		if subtree {
			scope = "subtree"
		} else {
			scope = "exact"
		}
	case "exact":
		subtree = false
	case "subtree":
		subtree = true
	default:
		render.Error(w, http.StatusBadRequest, "scope must be exact, subtree, or auto")
		return
	}

	tasks, err := h.app.Store.ListPublishTasksByMaterialRef(r.Context(), user.ID, deviceID, rootName, relativePath, subtree, limit)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load referencing tasks")
		return
	}

	diagnostics := make([]domain.PublishTaskDiagnosticItem, 0, len(tasks))
	for _, task := range tasks {
		diagnostic, diagErr := buildPublishTaskDiagnosticItem(r.Context(), h.app, user.ID, &task)
		if diagErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to build referencing task diagnostics")
			return
		}
		diagnostics = append(diagnostics, diagnostic)
	}

	taskSummary := summarizePublishTaskDiagnosticItems(diagnostics)
	render.JSON(w, http.StatusOK, domain.MaterialEntryWorkspace{
		DeviceID:         deviceID,
		Root:             *root,
		Entry:            *entry,
		Scope:            scope,
		ReferencingTasks: diagnostics,
		Summary: domain.MaterialImpactSummary{
			TaskCount:    taskSummary.TotalCount,
			ReadyCount:   taskSummary.ReadyCount,
			BlockedCount: taskSummary.BlockedCount,
			ByStatus:     taskSummary.ByStatus,
			ByDimension:  taskSummary.ByDimension,
			ByIssueCode:  taskSummary.ByIssueCode,
		},
	})
}

type syncMaterialRootsRequest struct {
	DeviceCode string `json:"deviceCode"`
	Roots      []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Exists      bool   `json:"exists"`
		IsDirectory bool   `json:"isDirectory"`
	} `json:"roots"`
}

type syncMaterialDirectoryRequest struct {
	DeviceCode   string `json:"deviceCode"`
	Root         string `json:"root"`
	RootPath     string `json:"rootPath"`
	Path         string `json:"path"`
	AbsolutePath string `json:"absolutePath"`
	Entries      []struct {
		Name         string  `json:"name"`
		Kind         string  `json:"kind"`
		RelativePath string  `json:"relativePath"`
		AbsolutePath string  `json:"absolutePath"`
		Size         *int64  `json:"size"`
		ModifiedAt   *string `json:"modifiedAt"`
		Extension    *string `json:"extension"`
		MimeType     *string `json:"mimeType"`
	} `json:"entries"`
}

type syncMaterialFileRequest struct {
	DeviceCode   string  `json:"deviceCode"`
	Root         string  `json:"root"`
	RootPath     string  `json:"rootPath"`
	Path         string  `json:"path"`
	AbsolutePath string  `json:"absolutePath"`
	Name         string  `json:"name"`
	Size         *int64  `json:"size"`
	ModifiedAt   *string `json:"modifiedAt"`
	Extension    *string `json:"extension"`
	MimeType     *string `json:"mimeType"`
	IsText       bool    `json:"isText"`
	Truncated    bool    `json:"truncated"`
	PreviewText  *string `json:"previewText"`
}

// 处理素材素材根目录同步接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) SyncRoots(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncMaterialRootsRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	if payload.DeviceCode == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode is required")
		return
	}

	device, ok := requireAgentDeviceByIdentity(w, r.Context(), h.app, payload.DeviceCode, agentKey, true, false)
	if !ok {
		return
	}

	items := make([]store.SyncMaterialRootInput, 0, len(payload.Roots))
	for _, item := range payload.Roots {
		name := strings.TrimSpace(item.Name)
		rootPath := strings.TrimSpace(item.Path)
		if name == "" || rootPath == "" {
			continue
		}
		items = append(items, store.SyncMaterialRootInput{
			DeviceID:    device.ID,
			RootName:    name,
			RootPath:    rootPath,
			IsAvailable: item.Exists,
			IsDirectory: item.IsDirectory,
		})
	}

	if err := h.app.Store.SyncMaterialRoots(r.Context(), device.ID, items); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync material roots")
		return
	}
	payloadBytes := 0
	if raw, marshalErr := json.Marshal(payload); marshalErr == nil {
		payloadBytes = len(raw)
	}
	if h.app.Logger != nil {
		h.app.Logger.Debug(
			"agent material roots synced",
			"device_id", device.ID,
			"device_code", device.DeviceCode,
			"root_count", len(items),
			"payload_bytes", payloadBytes,
		)
	}
	render.JSON(w, http.StatusOK, map[string]any{"synced": len(items)})
}

// 处理素材素材目录同步接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) SyncDirectory(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncMaterialDirectoryRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.Root = strings.TrimSpace(payload.Root)
	payload.RootPath = strings.TrimSpace(payload.RootPath)
	if payload.DeviceCode == "" || payload.Root == "" || payload.RootPath == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode, root, and rootPath are required")
		return
	}

	device, ok := requireAgentDeviceByIdentity(w, r.Context(), h.app, payload.DeviceCode, agentKey, true, false)
	if !ok {
		return
	}

	items := make([]store.SyncMaterialEntryInput, 0, len(payload.Entries))
	for _, item := range payload.Entries {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			kind = "file"
		}
		relativePath := normalizeMaterialPath(item.RelativePath)
		if relativePath == "" {
			relativePath = path.Join(normalizeMaterialPath(payload.Path), strings.TrimSpace(item.Name))
		}
		absolutePath := materialStringPtr(strings.TrimSpace(item.AbsolutePath))
		items = append(items, store.SyncMaterialEntryInput{
			DeviceID:     device.ID,
			RootName:     payload.Root,
			RootPath:     payload.RootPath,
			RelativePath: relativePath,
			ParentPath:   normalizeMaterialPath(payload.Path),
			Name:         strings.TrimSpace(item.Name),
			Kind:         kind,
			AbsolutePath: absolutePath,
			SizeBytes:    item.Size,
			ModifiedAt:   normalizeOptionalString(item.ModifiedAt),
			Extension:    normalizeOptionalString(item.Extension),
			MimeType:     normalizeOptionalString(item.MimeType),
			IsAvailable:  true,
		})
	}

	absolutePath := materialStringPtr(strings.TrimSpace(payload.AbsolutePath))
	if err := h.app.Store.SyncMaterialDirectory(r.Context(), device.ID, payload.Root, payload.RootPath, payload.Path, absolutePath, items); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync material directory")
		return
	}
	payloadBytes := 0
	if raw, marshalErr := json.Marshal(payload); marshalErr == nil {
		payloadBytes = len(raw)
	}
	if h.app.Logger != nil {
		h.app.Logger.Debug(
			"agent material directory synced",
			"device_id", device.ID,
			"device_code", device.DeviceCode,
			"root_name", payload.Root,
			"directory_path", payload.Path,
			"entry_count", len(items),
			"payload_bytes", payloadBytes,
		)
	}
	render.JSON(w, http.StatusOK, map[string]any{"synced": len(items)})
}

// 处理素材素材文件同步接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *MaterialHandler) SyncFile(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncMaterialFileRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.Root = strings.TrimSpace(payload.Root)
	payload.RootPath = strings.TrimSpace(payload.RootPath)
	payload.Path = normalizeMaterialPath(payload.Path)
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.DeviceCode == "" || payload.Root == "" || payload.RootPath == "" || payload.Path == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode, root, rootPath, and path are required")
		return
	}

	device, ok := requireAgentDeviceByIdentity(w, r.Context(), h.app, payload.DeviceCode, agentKey, true, false)
	if !ok {
		return
	}

	name := payload.Name
	if name == "" {
		name = path.Base(payload.Path)
	}
	previewText := normalizeOptionalString(payload.PreviewText)
	if previewText != nil && payload.Truncated {
		preview := *previewText + "\n[TRUNCATED]"
		previewText = &preview
	}

	item, err := h.app.Store.SyncMaterialFile(r.Context(), store.SyncMaterialEntryInput{
		DeviceID:     device.ID,
		RootName:     payload.Root,
		RootPath:     payload.RootPath,
		RelativePath: payload.Path,
		ParentPath:   normalizeMaterialParent(payload.Path),
		Name:         name,
		Kind:         "file",
		AbsolutePath: materialStringPtr(strings.TrimSpace(payload.AbsolutePath)),
		SizeBytes:    payload.Size,
		ModifiedAt:   normalizeOptionalString(payload.ModifiedAt),
		Extension:    normalizeOptionalString(payload.Extension),
		MimeType:     normalizeOptionalString(payload.MimeType),
		IsText:       payload.IsText,
		PreviewText:  previewText,
		IsAvailable:  true,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync material file")
		return
	}
	payloadBytes := 0
	if raw, marshalErr := json.Marshal(payload); marshalErr == nil {
		payloadBytes = len(raw)
	}
	if h.app.Logger != nil {
		h.app.Logger.Debug(
			"agent material file synced",
			"device_id", device.ID,
			"device_code", device.DeviceCode,
			"root_name", payload.Root,
			"relative_path", payload.Path,
			"payload_bytes", payloadBytes,
		)
	}

	render.JSON(w, http.StatusOK, item)
}

// 规范化可选String，统一素材链路的输入格式和后续处理行为。
func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 规范化素材路径，统一素材链路的输入格式和后续处理行为。
func normalizeMaterialPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == "/" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "/")
	return path.Clean(value)
}

// 规范化素材Parent，统一素材链路的输入格式和后续处理行为。
func normalizeMaterialParent(relativePath string) string {
	normalized := normalizeMaterialPath(relativePath)
	if normalized == "" {
		return ""
	}
	parent := path.Dir(normalized)
	if parent == "." || parent == "/" {
		return ""
	}
	return parent
}

// 处理素材StringPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func materialStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := strings.TrimSpace(value)
	return &v
}
