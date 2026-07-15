package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/response"
	"github.com/yang/wormhole_backend/internal/service"
)

// AnnouncementHandler 公告 HTTP 层。
type AnnouncementHandler struct {
	service service.AnnouncementService
}

// NewAnnouncementHandler 构造 AnnouncementHandler。
func NewAnnouncementHandler(svc service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{service: svc}
}

// ListVisible 返回当前用户可见公告。
// @Summary 获取已发布公告
// @Description 返回当前登录用户可见的已发布且未过期公告，置顶公告优先。
// @Tags announcements
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} response.APIResponse "公告列表"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 500 {object} dto.ErrorAPIResponse "获取公告失败"
// @Router /announcements [get]
func (h *AnnouncementHandler) ListVisible(c *gin.Context) {
	announcements, err := h.service.ListVisible(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err, "获取公告失败")
		return
	}
	response.Success(c, announcements)
}

// AdminList 管理员获取公告列表。
// @Summary 管理员获取公告列表
// @Description 返回全部公告，可按 status 筛选；草稿和过期公告也会返回。
// @Tags admin-announcements
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param status query int false "状态：1=发布，0=草稿/下架；不传返回全部"
// @Success 200 {object} response.APIResponse "公告列表"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "获取公告失败"
// @Router /admin/announcements [get]
func (h *AnnouncementHandler) AdminList(c *gin.Context) {
	status, err := optionalInt16Query(c, "status")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "status 参数错误", err.Error())
		return
	}
	announcements, err := h.service.AdminList(c.Request.Context(), status)
	if err != nil {
		h.writeServiceError(c, err, "获取公告失败")
		return
	}
	response.Success(c, announcements)
}

// AdminCreate 管理员新增公告。
// @Summary 管理员新增公告
// @Description 新增公告；发布时间为空时立即生效，expiresAt 为空时永不过期。
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param body body dto.CreateAnnouncementRequest true "公告"
// @Success 200 {object} response.APIResponse "公告"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 500 {object} dto.ErrorAPIResponse "新增公告失败"
// @Router /admin/announcements [post]
func (h *AnnouncementHandler) AdminCreate(c *gin.Context) {
	var req dto.CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	announcement, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err, "新增公告失败")
		return
	}
	response.Success(c, announcement)
}

// AdminUpdate 管理员编辑公告。
// @Summary 管理员编辑公告
// @Description 部分更新公告内容、置顶状态、发布时间、到期时间或发布状态。
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "公告 ID"
// @Param body body dto.UpdateAnnouncementRequest true "公告"
// @Success 200 {object} response.APIResponse "公告"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "公告不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "编辑公告失败"
// @Router /admin/announcements/{id} [patch]
func (h *AnnouncementHandler) AdminUpdate(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, 40001, "公告 ID 参数错误", nil)
		return
	}
	var req dto.UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	announcement, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err, "编辑公告失败")
		return
	}
	response.Success(c, announcement)
}

// AdminUpdateStatus 管理员发布或下架公告。
// @Summary 管理员更新公告状态
// @Description 更新公告状态，1=发布，0=草稿/下架。
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param id path int true "公告 ID"
// @Param body body dto.UpdateAnnouncementStatusRequest true "状态"
// @Success 200 {object} response.APIResponse "公告"
// @Failure 400 {object} dto.ErrorAPIResponse "参数错误"
// @Failure 401 {object} dto.ErrorAPIResponse "未登录或登录已失效"
// @Failure 403 {object} dto.ErrorAPIResponse "没有权限"
// @Failure 404 {object} dto.ErrorAPIResponse "公告不存在"
// @Failure 500 {object} dto.ErrorAPIResponse "更新公告状态失败"
// @Router /admin/announcements/{id}/status [patch]
func (h *AnnouncementHandler) AdminUpdateStatus(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, 40001, "公告 ID 参数错误", nil)
		return
	}
	var req dto.UpdateAnnouncementStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}
	announcement, err := h.service.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		h.writeServiceError(c, err, "更新公告状态失败")
		return
	}
	response.Success(c, announcement)
}

func (h *AnnouncementHandler) writeServiceError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrUnauthenticated):
		response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, 40301, "没有权限", nil)
	case errors.Is(err, service.ErrAnnouncementNotFound):
		response.Error(c, http.StatusNotFound, 40405, "公告不存在", nil)
	case errors.Is(err, service.ErrInvalidAnnouncement), errors.Is(err, service.ErrInvalidStatus):
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 50001, fallbackMessage, err.Error())
	}
}
