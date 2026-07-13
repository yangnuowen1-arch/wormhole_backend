package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/response"
	"github.com/yang/wormhole_backend/internal/service"
)

// ResourceHandler 资源中心 HTTP 层。
type ResourceHandler struct {
	resourceService       service.ResourceService
	searchHistoryService  service.SearchHistoryService
	commonToolService     service.CommonToolService
	quickEntryService     service.QuickEntryService
	recommendationService service.RecommendationItemService
	carouselSlideService  service.CarouselSlideService
}

// NewResourceHandler 构造 ResourceHandler。
func NewResourceHandler(resourceSvc service.ResourceService, searchHistorySvc service.SearchHistoryService, commonToolSvc service.CommonToolService, quickEntrySvc service.QuickEntryService, recommendationSvc service.RecommendationItemService, carouselSlideSvc service.CarouselSlideService) *ResourceHandler {
	return &ResourceHandler{
		resourceService:       resourceSvc,
		searchHistoryService:  searchHistorySvc,
		commonToolService:     commonToolSvc,
		quickEntryService:     quickEntrySvc,
		recommendationService: recommendationSvc,
		carouselSlideService:  carouselSlideSvc,
	}
}

// ListCategories 获取资源分类列表。
// @Summary 获取资源分类列表
// @Description 返回资源中心启用状态的分类，按 sort_order 升序排列。
// @Tags resources
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "资源分类列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取资源分类失败"
// @Router /resource-categories [get]
func (h *ResourceHandler) ListCategories(c *gin.Context) {
	categories, err := h.resourceService.ListCategories(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "获取资源分类失败", err.Error())
		return
	}
	response.Success(c, categories)
}

// ListResources 获取资源列表。
// @Summary 获取资源列表
// @Description 返回已发布资源列表，支持按分类、推荐状态和分页筛选。
// @Tags resources
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param category_id query int false "分类 ID"
// @Param category_code query string false "分类 code；all 或空表示全部"
// @Param featured query bool false "是否只看推荐资源"
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页数量，默认 20，最大 100"
// @Success 200 {object} response.APIResponse "资源分页列表"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取资源列表失败"
// @Router /resources [get]
func (h *ResourceHandler) ListResources(c *gin.Context) {
	page, pageSize := paginationFromQuery(c)
	categoryID, err := optionalInt32Query(c, "category_id", "categoryId")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "分类 ID 参数错误", err.Error())
		return
	}
	featured, err := optionalBoolQuery(c, "featured")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "featured 参数错误", err.Error())
		return
	}

	result, err := h.resourceService.ListResources(c.Request.Context(), service.ResourceListOptions{
		CategoryID:   categoryID,
		CategoryCode: stringQuery(c, "category_code", "categoryCode"),
		Featured:     featured,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "获取资源列表失败", err.Error())
		return
	}
	writePage(c, result.Items, result.Page, result.PageSize, result.Total)
}

// SearchResources 搜索资源。
// @Summary 搜索资源
// @Description 按关键词搜索已发布资源。该接口不写入搜索历史，前端可在搜索成功后调用 POST /search-history 记录。
// @Tags resources
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param q query string true "搜索关键词"
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页数量，默认 20，最大 100"
// @Success 200 {object} response.APIResponse "资源分页列表"
// @Failure 400 {object} dto.ErrorAPIResponse "搜索关键词不能为空"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "搜索资源失败"
// @Router /resources/search [get]
func (h *ResourceHandler) SearchResources(c *gin.Context) {
	page, pageSize := paginationFromQuery(c)
	result, err := h.resourceService.SearchResources(c.Request.Context(), c.Query("q"), page, pageSize)
	if err != nil {
		if errors.Is(err, service.ErrInvalidSearchQuery) {
			response.Error(c, http.StatusBadRequest, 40001, "搜索关键词不能为空", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "搜索资源失败", err.Error())
		return
	}
	writePage(c, result.Items, result.Page, result.PageSize, result.Total)
}

// GetResource 获取资源详情，identifier 支持 id 或 slug。
// @Summary 获取资源详情
// @Description 通过资源 ID 或 slug 获取已发布资源详情。
// @Tags resources
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param identifier path string true "资源 ID 或 slug"
// @Success 200 {object} response.APIResponse "资源详情"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 404 {object} dto.ErrorAPIResponse "资源不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "获取资源详情失败"
// @Router /resources/{identifier} [get]
func (h *ResourceHandler) GetResource(c *gin.Context) {
	resource, err := h.resourceService.GetResource(c.Request.Context(), c.Param("identifier"))
	if err != nil {
		if errors.Is(err, service.ErrResourceNotFound) {
			response.Error(c, http.StatusNotFound, 40401, "资源不存在", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "获取资源详情失败", err.Error())
		return
	}
	response.Success(c, resource)
}

// RecordSearchHistory 记录搜索历史。
// @Summary 记录搜索历史
// @Description 记录当前用户的一次搜索；同一用户同一关键词会累加 searchCount 并更新时间。
// @Tags search-history
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.RecordSearchHistoryRequest true "搜索历史记录"
// @Success 200 {object} response.APIResponse "搜索历史"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "记录搜索历史失败"
// @Router /search-history [post]
func (h *ResourceHandler) RecordSearchHistory(c *gin.Context) {
	var req dto.RecordSearchHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	history, err := h.searchHistoryService.Record(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidSearchQuery) {
			response.Error(c, http.StatusBadRequest, 40001, "搜索关键词不能为空", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "记录搜索历史失败", err.Error())
		return
	}
	response.Success(c, history)
}

// ListRecentSearchHistory 获取最近搜索历史。
// @Summary 获取最近搜索历史
// @Description 返回当前用户最近搜索历史，默认 4 条，最大 20 条。
// @Tags search-history
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param limit query int false "返回数量，默认 4，最大 20"
// @Success 200 {object} response.APIResponse "最近搜索历史"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取搜索历史失败"
// @Router /search-history/recent [get]
func (h *ResourceHandler) ListRecentSearchHistory(c *gin.Context) {
	limit := intQuery(c, 4, "limit")
	histories, err := h.searchHistoryService.ListRecent(c.Request.Context(), limit)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "获取搜索历史失败", err.Error())
		return
	}
	response.Success(c, histories)
}

// ClearSearchHistory 清空当前用户搜索历史。
// @Summary 清空搜索历史
// @Description 清空当前用户全部搜索历史。
// @Tags search-history
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "清空结果"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "清空搜索历史失败"
// @Router /search-history [delete]
func (h *ResourceHandler) ClearSearchHistory(c *gin.Context) {
	deleted, err := h.searchHistoryService.Clear(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "清空搜索历史失败", err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

// ListCommonTools 获取我的常用工具。
// @Summary 获取我的常用工具
// @Description 返回当前用户星标的常用工具，按个人排序展示。
// @Tags common-tools
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "常用工具列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取常用工具失败"
// @Router /common-tools [get]
func (h *ResourceHandler) ListCommonTools(c *gin.Context) {
	tools, err := h.commonToolService.List(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err, "获取常用工具失败")
		return
	}
	response.Success(c, tools)
}

// AddCommonTool 添加星标常用工具。
// @Summary 添加星标常用工具
// @Description 将一个资源加入当前用户常用工具；重复添加会更新排序值。
// @Tags common-tools
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.AddCommonToolRequest true "常用工具"
// @Success 200 {object} response.APIResponse "常用工具"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 404 {object} dto.ErrorAPIResponse "资源不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "添加常用工具失败"
// @Router /common-tools [post]
func (h *ResourceHandler) AddCommonTool(c *gin.Context) {
	var req dto.AddCommonToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	tool, err := h.commonToolService.Add(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err, "添加常用工具失败")
		return
	}
	response.Success(c, tool)
}

// RemoveCommonTool 取消星标常用工具。
// @Summary 取消星标常用工具
// @Description 从当前用户常用工具中移除指定资源。
// @Tags common-tools
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param resourceId path int true "资源 ID"
// @Success 200 {object} response.APIResponse "删除结果"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "取消常用工具失败"
// @Router /common-tools/{resourceId} [delete]
func (h *ResourceHandler) RemoveCommonTool(c *gin.Context) {
	resourceID, err := pathInt64(c, "resourceId")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "资源 ID 参数错误", err.Error())
		return
	}
	deleted, err := h.commonToolService.Remove(c.Request.Context(), resourceID)
	if err != nil {
		h.writeServiceError(c, err, "取消常用工具失败")
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

// SortCommonTools 更新我的常用工具排序。
// @Summary 更新我的常用工具排序
// @Description 批量更新当前用户常用工具排序。
// @Tags common-tools
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.SortCommonToolsRequest true "排序项"
// @Success 200 {object} response.APIResponse "排序结果"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "更新常用工具排序失败"
// @Router /common-tools/sort [put]
func (h *ResourceHandler) SortCommonTools(c *gin.Context) {
	var req dto.SortCommonToolsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	if err := h.commonToolService.Sort(c.Request.Context(), req); err != nil {
		h.writeServiceError(c, err, "更新常用工具排序失败")
		return
	}
	response.Success(c, gin.H{"sorted": true})
}

// ListQuickEntries 获取快速入口。
// @Summary 获取快速入口
// @Description 返回当前用户可见且启用的快速入口。
// @Tags quick-entries
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "快速入口列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取快速入口失败"
// @Router /quick-entries [get]
func (h *ResourceHandler) ListQuickEntries(c *gin.Context) {
	entries, err := h.quickEntryService.ListVisible(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err, "获取快速入口失败")
		return
	}
	response.Success(c, entries)
}

// AdminListQuickEntries 管理员获取快速入口列表。
// @Summary 管理员获取快速入口列表
// @Description 管理员查看全部快速入口，可按 status 过滤。
// @Tags admin-quick-entries
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param status query int false "状态：1=启用，0=停用；不传返回全部"
// @Success 200 {object} response.APIResponse "快速入口列表"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "获取快速入口失败"
// @Router /admin/quick-entries [get]
func (h *ResourceHandler) AdminListQuickEntries(c *gin.Context) {
	status, err := optionalInt16Query(c, "status")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "status 参数错误", err.Error())
		return
	}
	entries, err := h.quickEntryService.AdminList(c.Request.Context(), status)
	if err != nil {
		h.writeServiceError(c, err, "获取快速入口失败")
		return
	}
	response.Success(c, entries)
}

// AdminCreateQuickEntry 管理员新增快速入口。
// @Summary 管理员新增快速入口
// @Description 管理员新增一个快速入口。
// @Tags admin-quick-entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.CreateQuickEntryRequest true "快速入口"
// @Success 200 {object} response.APIResponse "快速入口"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "新增快速入口失败"
// @Router /admin/quick-entries [post]
func (h *ResourceHandler) AdminCreateQuickEntry(c *gin.Context) {
	var req dto.CreateQuickEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	entry, err := h.quickEntryService.Create(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err, "新增快速入口失败")
		return
	}
	response.Success(c, entry)
}

// AdminUpdateQuickEntry 管理员编辑快速入口。
// @Summary 管理员编辑快速入口
// @Description 管理员部分更新一个快速入口。
// @Tags admin-quick-entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "快速入口 ID"
// @Param body body dto.UpdateQuickEntryRequest true "快速入口"
// @Success 200 {object} response.APIResponse "快速入口"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "快速入口不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "编辑快速入口失败"
// @Router /admin/quick-entries/{id} [patch]
func (h *ResourceHandler) AdminUpdateQuickEntry(c *gin.Context) {
	id, err := pathInt32(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "快速入口 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateQuickEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	entry, err := h.quickEntryService.Update(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err, "编辑快速入口失败")
		return
	}
	response.Success(c, entry)
}

// AdminSortQuickEntries 管理员更新快速入口排序。
// @Summary 管理员更新快速入口排序
// @Description 管理员批量更新快速入口排序。
// @Tags admin-quick-entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.SortQuickEntriesRequest true "排序项"
// @Success 200 {object} response.APIResponse "排序结果"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "更新快速入口排序失败"
// @Router /admin/quick-entries/sort [put]
func (h *ResourceHandler) AdminSortQuickEntries(c *gin.Context) {
	var req dto.SortQuickEntriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	if err := h.quickEntryService.Sort(c.Request.Context(), req); err != nil {
		h.writeServiceError(c, err, "更新快速入口排序失败")
		return
	}
	response.Success(c, gin.H{"sorted": true})
}

// AdminUpdateQuickEntryStatus 管理员启停快速入口。
// @Summary 管理员启停快速入口
// @Description 管理员更新快速入口状态，1=启用，0=停用。
// @Tags admin-quick-entries
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "快速入口 ID"
// @Param body body dto.UpdateQuickEntryStatusRequest true "状态"
// @Success 200 {object} response.APIResponse "快速入口"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "快速入口不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "更新快速入口状态失败"
// @Router /admin/quick-entries/{id}/status [patch]
func (h *ResourceHandler) AdminUpdateQuickEntryStatus(c *gin.Context) {
	id, err := pathInt32(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "快速入口 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateQuickEntryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	entry, err := h.quickEntryService.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		h.writeServiceError(c, err, "更新快速入口状态失败")
		return
	}
	response.Success(c, entry)
}

// ListRecommendations 获取今日推荐。
// @Summary 获取今日推荐
// @Description 返回当前用户可见且启用的今日推荐，按 sort_order 升序排列。
// @Tags recommendations
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "今日推荐列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取今日推荐失败"
// @Router /recommendations [get]
func (h *ResourceHandler) ListRecommendations(c *gin.Context) {
	items, err := h.recommendationService.ListVisible(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err, "获取今日推荐失败")
		return
	}
	response.Success(c, items)
}

// AdminListRecommendations 管理员获取今日推荐列表。
// @Summary 管理员获取今日推荐列表
// @Description 管理员查看全部今日推荐，可按 status 过滤。
// @Tags admin-recommendations
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param status query int false "状态：1=启用，0=停用；不传返回全部"
// @Success 200 {object} response.APIResponse "今日推荐列表"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "获取今日推荐失败"
// @Router /admin/recommendations [get]
func (h *ResourceHandler) AdminListRecommendations(c *gin.Context) {
	status, err := optionalInt16Query(c, "status")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "status 参数错误", err.Error())
		return
	}
	items, err := h.recommendationService.AdminList(c.Request.Context(), status)
	if err != nil {
		h.writeServiceError(c, err, "获取今日推荐失败")
		return
	}
	response.Success(c, items)
}

// AdminCreateRecommendation 管理员新增今日推荐。
// @Summary 管理员新增今日推荐
// @Description 管理员新增一条今日推荐。
// @Tags admin-recommendations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.CreateRecommendationItemRequest true "今日推荐"
// @Success 200 {object} response.APIResponse "今日推荐"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "绑定资源不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "新增今日推荐失败"
// @Router /admin/recommendations [post]
func (h *ResourceHandler) AdminCreateRecommendation(c *gin.Context) {
	var req dto.CreateRecommendationItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	item, err := h.recommendationService.Create(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err, "新增今日推荐失败")
		return
	}
	response.Success(c, item)
}

// AdminUpdateRecommendation 管理员编辑今日推荐。
// @Summary 管理员编辑今日推荐
// @Description 管理员部分更新一条今日推荐。
// @Tags admin-recommendations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "今日推荐 ID"
// @Param body body dto.UpdateRecommendationItemRequest true "今日推荐"
// @Success 200 {object} response.APIResponse "今日推荐"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "今日推荐不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "编辑今日推荐失败"
// @Router /admin/recommendations/{id} [patch]
func (h *ResourceHandler) AdminUpdateRecommendation(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "今日推荐 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateRecommendationItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	item, err := h.recommendationService.Update(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err, "编辑今日推荐失败")
		return
	}
	response.Success(c, item)
}

// AdminSortRecommendations 管理员更新今日推荐排序。
// @Summary 管理员更新今日推荐排序
// @Description 管理员批量更新今日推荐排序。
// @Tags admin-recommendations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.SortRecommendationItemsRequest true "排序项"
// @Success 200 {object} response.APIResponse "排序结果"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "更新今日推荐排序失败"
// @Router /admin/recommendations/sort [put]
func (h *ResourceHandler) AdminSortRecommendations(c *gin.Context) {
	var req dto.SortRecommendationItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	if err := h.recommendationService.Sort(c.Request.Context(), req); err != nil {
		h.writeServiceError(c, err, "更新今日推荐排序失败")
		return
	}
	response.Success(c, gin.H{"sorted": true})
}

// AdminUpdateRecommendationStatus 管理员启停今日推荐。
// @Summary 管理员启停今日推荐
// @Description 管理员更新今日推荐状态，1=启用，0=停用。
// @Tags admin-recommendations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "今日推荐 ID"
// @Param body body dto.UpdateRecommendationItemStatusRequest true "状态"
// @Success 200 {object} response.APIResponse "今日推荐"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "今日推荐不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "更新今日推荐状态失败"
// @Router /admin/recommendations/{id}/status [patch]
func (h *ResourceHandler) AdminUpdateRecommendationStatus(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "今日推荐 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateRecommendationItemStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	item, err := h.recommendationService.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		h.writeServiceError(c, err, "更新今日推荐状态失败")
		return
	}
	response.Success(c, item)
}

// ListCarouselSlides 获取幻灯片。
// @Summary 获取幻灯片
// @Description 返回当前用户可见、启用且处于有效时间窗口内的幻灯片。
// @Tags carousel-slides
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "幻灯片列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取幻灯片失败"
// @Router /carousel-slides [get]
func (h *ResourceHandler) ListCarouselSlides(c *gin.Context) {
	slides, err := h.carouselSlideService.ListVisible(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err, "获取幻灯片失败")
		return
	}
	response.Success(c, slides)
}

// AdminListCarouselSlides 管理员获取幻灯片列表。
// @Summary 管理员获取幻灯片列表
// @Description 管理员查看全部幻灯片，可按 status 过滤。
// @Tags admin-carousel-slides
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param status query int false "状态：1=启用，0=停用；不传返回全部"
// @Success 200 {object} response.APIResponse "幻灯片列表"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "获取幻灯片失败"
// @Router /admin/carousel-slides [get]
func (h *ResourceHandler) AdminListCarouselSlides(c *gin.Context) {
	status, err := optionalInt16Query(c, "status")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "status 参数错误", err.Error())
		return
	}
	slides, err := h.carouselSlideService.AdminList(c.Request.Context(), status)
	if err != nil {
		h.writeServiceError(c, err, "获取幻灯片失败")
		return
	}
	response.Success(c, slides)
}

// AdminCreateCarouselSlide 管理员新增幻灯片。
// @Summary 管理员新增幻灯片
// @Description 管理员新增一张幻灯片。
// @Tags admin-carousel-slides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.CreateCarouselSlideRequest true "幻灯片"
// @Success 200 {object} response.APIResponse "幻灯片"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "新增幻灯片失败"
// @Router /admin/carousel-slides [post]
func (h *ResourceHandler) AdminCreateCarouselSlide(c *gin.Context) {
	var req dto.CreateCarouselSlideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	slide, err := h.carouselSlideService.Create(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err, "新增幻灯片失败")
		return
	}
	response.Success(c, slide)
}

// AdminUpdateCarouselSlide 管理员编辑幻灯片。
// @Summary 管理员编辑幻灯片
// @Description 管理员部分更新一张幻灯片。
// @Tags admin-carousel-slides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "幻灯片 ID"
// @Param body body dto.UpdateCarouselSlideRequest true "幻灯片"
// @Success 200 {object} response.APIResponse "幻灯片"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "幻灯片不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "编辑幻灯片失败"
// @Router /admin/carousel-slides/{id} [patch]
func (h *ResourceHandler) AdminUpdateCarouselSlide(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "幻灯片 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateCarouselSlideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	slide, err := h.carouselSlideService.Update(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err, "编辑幻灯片失败")
		return
	}
	response.Success(c, slide)
}

// AdminSortCarouselSlides 管理员更新幻灯片排序。
// @Summary 管理员更新幻灯片排序
// @Description 管理员批量更新幻灯片排序。
// @Tags admin-carousel-slides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.SortCarouselSlidesRequest true "排序项"
// @Success 200 {object} response.APIResponse "排序结果"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "更新幻灯片排序失败"
// @Router /admin/carousel-slides/sort [put]
func (h *ResourceHandler) AdminSortCarouselSlides(c *gin.Context) {
	var req dto.SortCarouselSlidesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	if err := h.carouselSlideService.Sort(c.Request.Context(), req); err != nil {
		h.writeServiceError(c, err, "更新幻灯片排序失败")
		return
	}
	response.Success(c, gin.H{"sorted": true})
}

// AdminUpdateCarouselSlideStatus 管理员启停幻灯片。
// @Summary 管理员启停幻灯片
// @Description 管理员更新幻灯片状态，1=启用，0=停用。
// @Tags admin-carousel-slides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "幻灯片 ID"
// @Param body body dto.UpdateCarouselSlideStatusRequest true "状态"
// @Success 200 {object} response.APIResponse "幻灯片"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "幻灯片不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "更新幻灯片状态失败"
// @Router /admin/carousel-slides/{id}/status [patch]
func (h *ResourceHandler) AdminUpdateCarouselSlideStatus(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "幻灯片 ID 参数错误", err.Error())
		return
	}
	var req dto.UpdateCarouselSlideStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	slide, err := h.carouselSlideService.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		h.writeServiceError(c, err, "更新幻灯片状态失败")
		return
	}
	response.Success(c, slide)
}

func (h *ResourceHandler) writeServiceError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrUnauthenticated):
		response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, 40301, "没有权限", nil)
	case errors.Is(err, service.ErrResourceNotFound):
		response.Error(c, http.StatusNotFound, 40401, "资源不存在", nil)
	case errors.Is(err, service.ErrQuickEntryNotFound):
		response.Error(c, http.StatusNotFound, 40402, "快速入口不存在", nil)
	case errors.Is(err, service.ErrRecommendationItemNotFound):
		response.Error(c, http.StatusNotFound, 40403, "今日推荐不存在", nil)
	case errors.Is(err, service.ErrCarouselSlideNotFound):
		response.Error(c, http.StatusNotFound, 40404, "幻灯片不存在", nil)
	case errors.Is(err, service.ErrInvalidQuickEntry), errors.Is(err, service.ErrInvalidStatus),
		errors.Is(err, service.ErrInvalidRecommendationItem), errors.Is(err, service.ErrInvalidCarouselSlide),
		errors.Is(err, service.ErrInvalidTimeRange):
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 50001, fallbackMessage, err.Error())
	}
}

func paginationFromQuery(c *gin.Context) (int, int) {
	return intQuery(c, 1, "page"), intQuery(c, 20, "pageSize", "page_size")
}

func intQuery(c *gin.Context, fallback int, keys ...string) int {
	raw := stringQuery(c, keys...)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func optionalInt32Query(c *gin.Context, keys ...string) (*int32, error) {
	raw := stringQuery(c, keys...)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return nil, err
	}
	v := int32(value)
	return &v, nil
}

func optionalInt16Query(c *gin.Context, keys ...string) (*int16, error) {
	raw := stringQuery(c, keys...)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 16)
	if err != nil {
		return nil, err
	}
	v := int16(value)
	return &v, nil
}

func stringQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func pathInt64(c *gin.Context, key string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(c.Param(key)), 10, 64)
}

func pathInt32(c *gin.Context, key string) (int32, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param(key)), 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(value), nil
}

func optionalBoolQuery(c *gin.Context, key string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func writePage(c *gin.Context, items any, page, pageSize int, total int64) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	response.Success(c, response.PageResult{
		Items: items,
		Pagination: response.Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}
