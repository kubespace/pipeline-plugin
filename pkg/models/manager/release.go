package manager

import (
	"github.com/kubespace/pipeline-plugin/pkg/models/types"
	"gorm.io/gorm"
	"time"
)

type Release struct {
	DB *gorm.DB
}

func NewReleaseManager(db *gorm.DB) *Release {
	return &Release{DB: db}
}

func (l *Release) Add(workspaceId uint, version string, jobRunId uint) error {
	var cnt int64
	if err := l.DB.Model(types.PipelineWorkspaceRelease{}).Where("job_run_id = ? and release_version = ?", jobRunId, version).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	var release = types.PipelineWorkspaceRelease{
		WorkspaceId:    workspaceId,
		ReleaseVersion: version,
		JobRunId:       jobRunId,
		CreateTime:     time.Now(),
		UpdateTime:     time.Now(),
	}
	if err := l.DB.Create(&release).Error; err != nil {
		return err
	}
	return nil
}
