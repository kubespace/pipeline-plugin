package router

import (
	"github.com/kubespace/pipeline-plugin/pkg/views"
)

type ViewSets map[string][]*views.View

func NewViewSets() *ViewSets {
	plugins := views.NewPluginViews()
	return &ViewSets{
		"plugin": plugins.Views,
	}
}
