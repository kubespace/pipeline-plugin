package types

import "time"

type PipelineRunJobLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	JobRunId   uint      `gorm:"column:job_run_id;not null" json:"job_run_id"`
	Logs       string    `gorm:"type:longtext" json:"logs"`
	CreateTime time.Time `gorm:"not null;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"not null;autoUpdateTime" json:"update_time"`
}
