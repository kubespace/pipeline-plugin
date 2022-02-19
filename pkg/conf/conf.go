package conf

import "github.com/kubespace/pipeline-plugin/pkg/utils"

type GlobalConf struct {
	DataDir          string
	CallbackEndpoint string
	CallbackUrl      string
	CallbackClient   *utils.HttpClient
}

var AppConfig *GlobalConf = &GlobalConf{}
