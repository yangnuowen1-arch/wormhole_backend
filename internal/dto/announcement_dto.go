package dto

// AnnouncementResponse 公告响应。
type AnnouncementResponse struct {
	ID          int64  `json:"id" example:"1"`
	Title       string `json:"title" example:"平台维护通知"`
	Content     string `json:"content" example:"本周六 02:00-03:00 进行例行维护。"`
	IsPinned    bool   `json:"isPinned" example:"true"`
	Status      int16  `json:"status" example:"1"`
	PublishedAt string `json:"publishedAt" example:"2026-07-13T10:00:00Z"`
	ExpiresAt   string `json:"expiresAt" example:"2026-07-20T10:00:00Z"`
	CreatedAt   string `json:"createdAt" example:"2026-07-13T10:00:00Z"`
	UpdatedAt   string `json:"updatedAt" example:"2026-07-13T10:00:00Z"`
}

// CreateAnnouncementRequest 新增公告请求。
type CreateAnnouncementRequest struct {
	Title       string `json:"title" binding:"required,max=128" example:"平台维护通知"`
	Content     string `json:"content" binding:"required,max=10000" example:"本周六 02:00-03:00 进行例行维护。"`
	IsPinned    bool   `json:"isPinned" example:"false"`
	Status      *int16 `json:"status" example:"1"`
	PublishedAt string `json:"publishedAt" example:"2026-07-13T10:00:00Z"`
	ExpiresAt   string `json:"expiresAt" example:"2026-07-20T10:00:00Z"`
}

// UpdateAnnouncementRequest 编辑公告请求。
type UpdateAnnouncementRequest struct {
	Title       *string `json:"title" binding:"omitempty,max=128" example:"平台维护通知"`
	Content     *string `json:"content" binding:"omitempty,max=10000" example:"本周六 02:00-03:00 进行例行维护。"`
	IsPinned    *bool   `json:"isPinned" example:"false"`
	Status      *int16  `json:"status" example:"1"`
	PublishedAt *string `json:"publishedAt" example:"2026-07-13T10:00:00Z"`
	ExpiresAt   *string `json:"expiresAt" example:"2026-07-20T10:00:00Z"`
}

// UpdateAnnouncementStatusRequest 更新公告发布状态请求。
type UpdateAnnouncementStatusRequest struct {
	Status int16 `json:"status" example:"1"`
}
