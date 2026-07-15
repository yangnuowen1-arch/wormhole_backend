package model

import "time"

// TableNameAnnouncement 是系统公告表名。
const TableNameAnnouncement = "announcements"

// AnnouncementRecord 是公告表的手写映射。
//
// 名称特意使用 Record 后缀，避免日后运行 gorm/gen 时与自动生成的
// Announcement 模型重名。
type AnnouncementRecord struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title       string     `gorm:"column:title;type:character varying(128);not null" json:"title"`
	Content     string     `gorm:"column:content;type:text;not null" json:"content"`
	IsPinned    bool       `gorm:"column:is_pinned;not null;default:false" json:"is_pinned"`
	Status      int16      `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
	PublishedAt time.Time  `gorm:"column:published_at;type:timestamp without time zone;not null" json:"published_at"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;type:timestamp without time zone" json:"expires_at"`
	CreatedBy   *int64     `gorm:"column:created_by;type:bigint" json:"created_by"`
	UpdatedBy   *int64     `gorm:"column:updated_by;type:bigint" json:"updated_by"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamp without time zone;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamp without time zone;not null" json:"updated_at"`
}

// TableName 返回公告表名。
func (*AnnouncementRecord) TableName() string {
	return TableNameAnnouncement
}
