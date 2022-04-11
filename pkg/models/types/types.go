package types

import "time"

type PipelineRunJobLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	JobRunId   uint      `gorm:"column:job_run_id;not null" json:"job_run_id"`
	Logs       string    `gorm:"type:longtext" json:"logs"`
	CreateTime time.Time `gorm:"not null;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"not null;autoUpdateTime" json:"update_time"`
}

type PipelineWorkspaceRelease struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceId    uint      `gorm:"not null;uniqueIndex:idx_workspace_version" json:"workspace_id"`
	ReleaseVersion string    `gorm:"size:500;not null;uniqueIndex:idx_workspace_version" json:"release_version"`
	JobRunId       uint      `gorm:"not null;" json:"job_run_id"`
	CreateTime     time.Time `gorm:"column:create_time;not null;autoCreateTime" json:"create_time"`
	UpdateTime     time.Time `gorm:"column:update_time;not null;autoUpdateTime" json:"update_time"`
}
