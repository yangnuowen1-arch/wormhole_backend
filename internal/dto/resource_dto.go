package dto

// ResourceCategoryResponse 资源分类响应。
type ResourceCategoryResponse struct {
	ID          int32  `json:"id" example:"1"`
	Code        string `json:"code" example:"ai"`
	Name        string `json:"name" example:"AI 与机器学习"`
	Description string `json:"description" example:"AI 工具与模型资源"`
	SortOrder   int32  `json:"sortOrder" example:"10"`
}

// ResourceCategorySummary 资源分类摘要。
type ResourceCategorySummary struct {
	ID   int32  `json:"id" example:"1"`
	Code string `json:"code" example:"ai"`
	Name string `json:"name" example:"AI 与机器学习"`
}

// ResourceResponse 资源响应。
type ResourceResponse struct {
	ID            int64                    `json:"id" example:"1"`
	Category      *ResourceCategorySummary `json:"category"`
	Slug          string                   `json:"slug" example:"claude"`
	Name          string                   `json:"name" example:"Claude"`
	IconURL       string                   `json:"iconUrl" example:"https://example.com/claude.png"`
	IconText      string                   `json:"iconText" example:"Cl"`
	WebsiteURL    string                   `json:"websiteUrl" example:"https://claude.ai"`
	Summary       string                   `json:"summary" example:"AI 助手"`
	Description   string                   `json:"description" example:"企业资源与模型生态入口"`
	ResourceType  string                   `json:"resourceType" example:"tool"`
	Provider      string                   `json:"provider" example:"Anthropic"`
	ModelCount    int32                    `json:"modelCount" example:"12"`
	FollowerCount int32                    `json:"followerCount" example:"968"`
	Badge         string                   `json:"badge" example:"Enterprise"`
	Tags          []string                 `json:"tags"`
	Metadata      map[string]any           `json:"metadata"`
	IsFeatured    bool                     `json:"isFeatured" example:"true"`
	SortOrder     int32                    `json:"sortOrder" example:"10"`
}

// CommonToolResponse 常用工具响应。
type CommonToolResponse struct {
	Resource  ResourceResponse `json:"resource"`
	SortOrder int32            `json:"sortOrder" example:"10"`
	StarredAt string           `json:"starredAt" example:"2026-07-12T11:30:00Z"`
}

// AddCommonToolRequest 添加常用工具请求。
type AddCommonToolRequest struct {
	ResourceID int64 `json:"resourceId" binding:"required" example:"1"`
	SortOrder  int32 `json:"sortOrder" example:"10"`
}

// CommonToolSortItem 常用工具排序项。
type CommonToolSortItem struct {
	ResourceID int64 `json:"resourceId" binding:"required" example:"1"`
	SortOrder  int32 `json:"sortOrder" example:"10"`
}

// SortCommonToolsRequest 常用工具排序请求。
type SortCommonToolsRequest struct {
	Items []CommonToolSortItem `json:"items" binding:"required,dive"`
}

// QuickEntryResponse 快速入口响应。
type QuickEntryResponse struct {
	ID               int32    `json:"id" example:"1"`
	Code             string   `json:"code" example:"github"`
	Title            string   `json:"title" example:"GitHub"`
	IconURL          string   `json:"iconUrl" example:"https://example.com/github.png"`
	IconText         string   `json:"iconText" example:"Git"`
	TargetURL        string   `json:"targetUrl" example:"https://github.com"`
	Description      string   `json:"description" example:"代码托管平台"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           int16    `json:"status" example:"1"`
}

// CreateQuickEntryRequest 新增快速入口请求。
type CreateQuickEntryRequest struct {
	Code             string   `json:"code" binding:"required,max=64" example:"github"`
	Title            string   `json:"title" binding:"required,max=64" example:"GitHub"`
	IconURL          string   `json:"iconUrl" binding:"omitempty,max=255" example:"https://example.com/github.png"`
	IconText         string   `json:"iconText" binding:"omitempty,max=16" example:"Git"`
	TargetURL        string   `json:"targetUrl" binding:"required,max=512" example:"https://github.com"`
	Description      string   `json:"description" binding:"omitempty,max=255" example:"代码托管平台"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// UpdateQuickEntryRequest 编辑快速入口请求。
type UpdateQuickEntryRequest struct {
	Code             *string  `json:"code" binding:"omitempty,max=64" example:"github"`
	Title            *string  `json:"title" binding:"omitempty,max=64" example:"GitHub"`
	IconURL          *string  `json:"iconUrl" binding:"omitempty,max=255" example:"https://example.com/github.png"`
	IconText         *string  `json:"iconText" binding:"omitempty,max=16" example:"Git"`
	TargetURL        *string  `json:"targetUrl" binding:"omitempty,max=512" example:"https://github.com"`
	Description      *string  `json:"description" binding:"omitempty,max=255" example:"代码托管平台"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        *int32   `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// QuickEntrySortItem 快速入口排序项。
type QuickEntrySortItem struct {
	ID        int32 `json:"id" binding:"required" example:"1"`
	SortOrder int32 `json:"sortOrder" example:"10"`
}

// SortQuickEntriesRequest 快速入口排序请求。
type SortQuickEntriesRequest struct {
	Items []QuickEntrySortItem `json:"items" binding:"required,dive"`
}

// UpdateQuickEntryStatusRequest 更新快速入口状态请求。
type UpdateQuickEntryStatusRequest struct {
	Status int16 `json:"status" example:"1"`
}

// RecommendationItemResponse 今日推荐响应。
type RecommendationItemResponse struct {
	ID               int64    `json:"id" example:"1"`
	ResourceID       *int64   `json:"resourceId" example:"1"`
	Title            string   `json:"title" example:"Claude 3.5：工程师需要了解的要点"`
	Subtitle         string   `json:"subtitle" example:"Anthropic"`
	SourceName       string   `json:"sourceName" example:"Anthropic"`
	SourceURL        string   `json:"sourceUrl" example:"https://www.anthropic.com"`
	IconURL          string   `json:"iconUrl" example:"https://example.com/icon.png"`
	IconText         string   `json:"iconText" example:"Cl"`
	TargetURL        string   `json:"targetUrl" example:"https://example.com/article"`
	PublishedAt      string   `json:"publishedAt" example:"2026-07-12T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           int16    `json:"status" example:"1"`
}

// CreateRecommendationItemRequest 新增今日推荐请求。
type CreateRecommendationItemRequest struct {
	ResourceID       *int64   `json:"resourceId" example:"1"`
	Title            string   `json:"title" binding:"required,max=128" example:"Claude 3.5：工程师需要了解的要点"`
	Subtitle         string   `json:"subtitle" binding:"omitempty,max=128" example:"Anthropic"`
	SourceName       string   `json:"sourceName" binding:"omitempty,max=64" example:"Anthropic"`
	SourceURL        string   `json:"sourceUrl" binding:"omitempty,max=512" example:"https://www.anthropic.com"`
	IconURL          string   `json:"iconUrl" binding:"omitempty,max=255" example:"https://example.com/icon.png"`
	IconText         string   `json:"iconText" binding:"omitempty,max=16" example:"Cl"`
	TargetURL        string   `json:"targetUrl" binding:"omitempty,max=512" example:"https://example.com/article"`
	PublishedAt      string   `json:"publishedAt" example:"2026-07-12T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// UpdateRecommendationItemRequest 编辑今日推荐请求。
type UpdateRecommendationItemRequest struct {
	ResourceID       *int64   `json:"resourceId" example:"1"`
	Title            *string  `json:"title" binding:"omitempty,max=128" example:"Claude 3.5：工程师需要了解的要点"`
	Subtitle         *string  `json:"subtitle" binding:"omitempty,max=128" example:"Anthropic"`
	SourceName       *string  `json:"sourceName" binding:"omitempty,max=64" example:"Anthropic"`
	SourceURL        *string  `json:"sourceUrl" binding:"omitempty,max=512" example:"https://www.anthropic.com"`
	IconURL          *string  `json:"iconUrl" binding:"omitempty,max=255" example:"https://example.com/icon.png"`
	IconText         *string  `json:"iconText" binding:"omitempty,max=16" example:"Cl"`
	TargetURL        *string  `json:"targetUrl" binding:"omitempty,max=512" example:"https://example.com/article"`
	PublishedAt      *string  `json:"publishedAt" example:"2026-07-12T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        *int32   `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// RecommendationItemSortItem 今日推荐排序项。
type RecommendationItemSortItem struct {
	ID        int64 `json:"id" binding:"required" example:"1"`
	SortOrder int32 `json:"sortOrder" example:"10"`
}

// SortRecommendationItemsRequest 今日推荐排序请求。
type SortRecommendationItemsRequest struct {
	Items []RecommendationItemSortItem `json:"items" binding:"required,dive"`
}

// UpdateRecommendationItemStatusRequest 更新今日推荐状态请求。
type UpdateRecommendationItemStatusRequest struct {
	Status int16 `json:"status" example:"1"`
}

// CarouselSlideResponse 幻灯片响应。
type CarouselSlideResponse struct {
	ID               int64    `json:"id" example:"1"`
	Code             string   `json:"code" example:"ai-console"`
	Title            string   `json:"title" example:"AI 助手控制台"`
	Subtitle         string   `json:"subtitle" example:"看板"`
	Description      string   `json:"description" example:"重塑工具、对比性能、生成报告。"`
	ImageURL         string   `json:"imageUrl" example:"https://example.com/banner.png"`
	Background       string   `json:"background" example:"#6D28D9"`
	ButtonText       string   `json:"buttonText" example:"立即查看"`
	TargetURL        string   `json:"targetUrl" example:"https://example.com/ai-console"`
	AutoplaySeconds  int32    `json:"autoplaySeconds" example:"5"`
	StartsAt         string   `json:"startsAt" example:"2026-07-12T11:30:00Z"`
	EndsAt           string   `json:"endsAt" example:"2026-07-19T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           int16    `json:"status" example:"1"`
}

// CreateCarouselSlideRequest 新增幻灯片请求。
type CreateCarouselSlideRequest struct {
	Code             string   `json:"code" binding:"required,max=64" example:"ai-console"`
	Title            string   `json:"title" binding:"required,max=128" example:"AI 助手控制台"`
	Subtitle         string   `json:"subtitle" binding:"omitempty,max=128" example:"看板"`
	Description      string   `json:"description" binding:"omitempty,max=512" example:"重塑工具、对比性能、生成报告。"`
	ImageURL         string   `json:"imageUrl" binding:"omitempty,max=512" example:"https://example.com/banner.png"`
	Background       string   `json:"background" binding:"omitempty,max=64" example:"#6D28D9"`
	ButtonText       string   `json:"buttonText" binding:"omitempty,max=32" example:"立即查看"`
	TargetURL        string   `json:"targetUrl" binding:"omitempty,max=512" example:"https://example.com/ai-console"`
	AutoplaySeconds  *int32   `json:"autoplaySeconds" example:"5"`
	StartsAt         string   `json:"startsAt" example:"2026-07-12T11:30:00Z"`
	EndsAt           string   `json:"endsAt" example:"2026-07-19T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        int32    `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// UpdateCarouselSlideRequest 编辑幻灯片请求。
type UpdateCarouselSlideRequest struct {
	Code             *string  `json:"code" binding:"omitempty,max=64" example:"ai-console"`
	Title            *string  `json:"title" binding:"omitempty,max=128" example:"AI 助手控制台"`
	Subtitle         *string  `json:"subtitle" binding:"omitempty,max=128" example:"看板"`
	Description      *string  `json:"description" binding:"omitempty,max=512" example:"重塑工具、对比性能、生成报告。"`
	ImageURL         *string  `json:"imageUrl" binding:"omitempty,max=512" example:"https://example.com/banner.png"`
	Background       *string  `json:"background" binding:"omitempty,max=64" example:"#6D28D9"`
	ButtonText       *string  `json:"buttonText" binding:"omitempty,max=32" example:"立即查看"`
	TargetURL        *string  `json:"targetUrl" binding:"omitempty,max=512" example:"https://example.com/ai-console"`
	AutoplaySeconds  *int32   `json:"autoplaySeconds" example:"5"`
	StartsAt         *string  `json:"startsAt" example:"2026-07-12T11:30:00Z"`
	EndsAt           *string  `json:"endsAt" example:"2026-07-19T11:30:00Z"`
	VisibleRoleCodes []string `json:"visibleRoleCodes"`
	SortOrder        *int32   `json:"sortOrder" example:"10"`
	Status           *int16   `json:"status" example:"1"`
}

// CarouselSlideSortItem 幻灯片排序项。
type CarouselSlideSortItem struct {
	ID        int64 `json:"id" binding:"required" example:"1"`
	SortOrder int32 `json:"sortOrder" example:"10"`
}

// SortCarouselSlidesRequest 幻灯片排序请求。
type SortCarouselSlidesRequest struct {
	Items []CarouselSlideSortItem `json:"items" binding:"required,dive"`
}

// UpdateCarouselSlideStatusRequest 更新幻灯片状态请求。
type UpdateCarouselSlideStatusRequest struct {
	Status int16 `json:"status" example:"1"`
}

// RecordSearchHistoryRequest 记录搜索历史请求。
type RecordSearchHistoryRequest struct {
	Query           string `json:"query" binding:"required,max=128" example:"Claude"`
	LastResultCount int32  `json:"lastResultCount" example:"8"`
}

// SearchHistoryResponse 搜索历史响应。
type SearchHistoryResponse struct {
	ID              int64  `json:"id" example:"1"`
	Query           string `json:"query" example:"Claude"`
	SearchCount     int32  `json:"searchCount" example:"3"`
	LastResultCount int32  `json:"lastResultCount" example:"8"`
	LastSearchedAt  string `json:"lastSearchedAt" example:"2026-07-12T11:30:00Z"`
}
