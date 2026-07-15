package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/service"
)

func TestAnnouncementListVisibleReturnsAnnouncements(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAnnouncementHandler(&announcementServiceStub{
		listVisible: func(context.Context) ([]dto.AnnouncementResponse, error) {
			return []dto.AnnouncementResponse{{
				ID:       1,
				Title:    "平台维护通知",
				Content:  "本周六维护。",
				IsPinned: true,
			}}, nil
		},
	})
	router := gin.New()
	router.GET("/api/v1/announcements", h.ListVisible)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var body struct {
		Code int                        `json:"code"`
		Data []dto.AnnouncementResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || len(body.Data) != 1 || body.Data[0].Title != "平台维护通知" || !body.Data[0].IsPinned {
		t.Fatalf("response = %+v, want pinned announcement", body)
	}
}

type announcementServiceStub struct {
	listVisible func(context.Context) ([]dto.AnnouncementResponse, error)
}

func (s *announcementServiceStub) ListVisible(ctx context.Context) ([]dto.AnnouncementResponse, error) {
	if s.listVisible == nil {
		return nil, nil
	}
	return s.listVisible(ctx)
}

func (s *announcementServiceStub) AdminList(ctx context.Context, status *int16) ([]dto.AnnouncementResponse, error) {
	return nil, nil
}

func (s *announcementServiceStub) Create(ctx context.Context, req dto.CreateAnnouncementRequest) (dto.AnnouncementResponse, error) {
	return dto.AnnouncementResponse{}, nil
}

func (s *announcementServiceStub) Update(ctx context.Context, id int64, req dto.UpdateAnnouncementRequest) (dto.AnnouncementResponse, error) {
	return dto.AnnouncementResponse{}, nil
}

func (s *announcementServiceStub) UpdateStatus(ctx context.Context, id int64, status int16) (dto.AnnouncementResponse, error) {
	return dto.AnnouncementResponse{}, nil
}

var _ service.AnnouncementService = (*announcementServiceStub)(nil)
