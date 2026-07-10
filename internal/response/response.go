package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一响应体。
type APIResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	Error     any    `json:"error"`
	RequestID string `json:"requestId"`
	Timestamp string `json:"timestamp"`
}

// Pagination 分页信息。
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// PageResult 列表分页返回。
type PageResult struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Success 返回 200 成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(200, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// Created 返回 201 创建成功响应。
func Created(c *gin.Context, data any) {
	c.JSON(201, APIResponse{
		Code:      0,
		Message:   "created",
		Data:      data,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// Error 返回错误响应，httpStatus 为 HTTP 状态码，code 为业务错误码。
func Error(c *gin.Context, httpStatus, code int, message string, errData any) {
	c.JSON(httpStatus, APIResponse{
		Code:      code,
		Message:   message,
		Error:     errData,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
