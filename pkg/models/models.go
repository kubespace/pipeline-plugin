package models

import (
	"github.com/kubespace/pipeline-plugin/pkg/models/manager"
	"github.com/kubespace/pipeline-plugin/pkg/models/mysql"
)

type models struct {
	JobLogManager          *manager.JobLog
	PipelineReleaseManager *manager.Release
}

var Models *models

func NewModels(mysqlOptions *mysql.Options) (*models, error) {
	db, err := mysql.NewMysqlDb(mysqlOptions)
	if err != nil {
		return nil, err
	}
	jobLog := manager.NewJobLogManager(db)
	release := manager.NewReleaseManager(db)
	return &models{
		JobLogManager:          jobLog,
		PipelineReleaseManager: release,
	}, nil
}
