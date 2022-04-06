package plugins

import (
	"encoding/json"
	"fmt"
	"github.com/kubespace/pipeline-plugin/pkg/conf"
	"github.com/kubespace/pipeline-plugin/pkg/models"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"github.com/kubespace/pipeline-plugin/pkg/utils/code"
	"io"
	"k8s.io/klog"
	"os"
	"time"
)

const (
	PluginBuildCodeToImage = "build_code_to_image"
)

type PluginExecutor interface {
	execute() (interface{}, error)
}

type BasePlugin struct {
	PluginType string
	RootDir    string
	LogFile    string
	CloseLog   chan struct{}
	JobId      uint
	Executor   PluginExecutor
	Logger     io.Writer
}

func NewBasePlugin(jobId uint, pluginType string) *BasePlugin {
	rootDir := fmt.Sprintf("%s/%d", conf.AppConfig.DataDir, jobId)
	logFile := fmt.Sprintf(rootDir + "/.klog")
	return &BasePlugin{
		RootDir:    rootDir,
		JobId:      jobId,
		LogFile:    logFile,
		PluginType: pluginType,
		CloseLog:   make(chan struct{}),
	}
}

func (b *BasePlugin) InitRootDir(pluginType string, pluginParams interface{}) error {
	klog.Infof("job=%d make root dir %s of plugin %s", b.JobId, b.RootDir, pluginType)
	err := os.MkdirAll(b.RootDir, 0755)
	if err != nil {
		klog.Errorf("job=%d mkdir %s error: %v", b.JobId, b.RootDir, err)
		return fmt.Errorf("mkdir %s error: %v", b.RootDir, err)
	}
	metadata := map[string]interface{}{
		"plugin_type": pluginType,
		"params":      pluginParams,
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		klog.Errorf("job=%d marshal plugin metadata error: %v", b.JobId, err)
		return fmt.Errorf("marshal plugin metadata error: %v", err)
	}
	metadataFile, err := os.Create(b.RootDir + "/.metadata")
	defer metadataFile.Close()
	if err != nil && !os.IsExist(err) {
		klog.Errorf("job=%d create metadata file error: %v", b.JobId, err)
		return fmt.Errorf("create metadata file error: %v", err)
	}
	if _, err = metadataFile.Write(metadataBytes); err != nil {
		klog.Errorf("job=%d write metadata error: %v", b.JobId, err)
		return fmt.Errorf("write metadata error: %v", err)
	}
	os.RemoveAll(b.LogFile)
	return nil
}

func (b *BasePlugin) Clear() {
	if err := os.RemoveAll(b.RootDir); err != nil {
		klog.Errorf("job=%d remove root dir %s error: %v", b.JobId, b.RootDir, err)
	}
}

func (b *BasePlugin) Log(format string, a ...interface{}) {
	_, err := fmt.Fprintf(b.Logger, format+"\n", a...)
	if err != nil {
		klog.Errorf("job=%d write log error: %v", b.JobId, err)
	}
}

func (b *BasePlugin) FlushLogToDB() {
	tick := time.NewTicker(5 * time.Second)
	logStat, err := os.Stat(b.LogFile)
	if err != nil {
		klog.Errorf("stat log file %s error %s", b.LogFile, err.Error())
	}
	for {
		klog.Infof("update log")
		select {
		case <-b.CloseLog:
			klog.Info("close log update")
			err := models.Models.JobLogManager.UpdateLog(b.JobId, b.LogFile)
			if err != nil {
				klog.Errorf("update job %s log error: %s", b.JobId, err.Error())
			}
			return
		case <-tick.C:
			//c.SSEvent("message", event)
			currLogStat, err := os.Stat(b.LogFile)
			if err != nil {
				klog.Errorf("stat log file %s error %s", b.LogFile, err.Error())
			} else {
				if logStat == nil || currLogStat.ModTime() != logStat.ModTime() {
					err := models.Models.JobLogManager.UpdateLog(b.JobId, b.LogFile)
					if err != nil {
						klog.Errorf("update job %s log error: %s", b.JobId, err.Error())
					}
				}
			}
		}
	}
}

func (b *BasePlugin) Execute(pluginParams interface{}) {
	err := b.InitRootDir(b.PluginType, pluginParams)
	//defer b.Clear()
	if err != nil {
		b.Callback(&utils.Response{Code: code.InitError, Msg: err.Error()})
		return
	}
	if b.Executor == nil {
		b.Callback(&utils.Response{Code: code.ExecError, Msg: "not found plugin execute function"})
		return
	}
	logFile, err := os.OpenFile(b.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer logFile.Close()
	if err != nil {
		b.Callback(&utils.Response{Code: code.InitError, Msg: "open log file error: " + err.Error()})
		return
	}
	b.Logger = logFile
	go b.FlushLogToDB()
	defer close(b.CloseLog)
	result, err := b.Executor.execute()
	if err != nil {
		b.Callback(&utils.Response{Code: code.ExecError, Msg: err.Error()})
		return
	}
	b.Callback(&utils.Response{Code: code.Success, Data: result})
}

func (b *BasePlugin) Callback(resp *utils.Response) {
	klog.Infof("job=%d callback response: %v", b.JobId, resp)
	data := map[string]interface{}{
		"job_id": b.JobId,
		"result": resp,
	}
	dataBytes, _ := json.Marshal(data)
	ret, err := conf.AppConfig.CallbackClient.Post(conf.AppConfig.CallbackUrl, nil, dataBytes)
	if err != nil {
		klog.Errorf("job=%d callback to pipeline error: %v", b.JobId, err)
		return
	}
	klog.Infof("job=%d callback to pipeline return: %s", b.JobId, string(ret))
}
