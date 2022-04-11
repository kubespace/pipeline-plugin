package plugins

import (
	"fmt"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"github.com/kubespace/pipeline-plugin/pkg/utils/code"
	"github.com/kubespace/pipeline-plugin/pkg/views/serializers"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	"os/exec"
)

const (
	ResourceTypeImage = "image"
	ResourceTypeHost  = "host"
)

type ExecShell struct{}

func NewExecShell() *ExecShell {
	return &ExecShell{}
}

func (b *ExecShell) ExecuteShell(ser *serializers.ExecShellSerializer) *utils.Response {
	shellPlugin, err := NewExecShellPlugin(ser)
	if err != nil {
		return &utils.Response{Code: code.InitError, Msg: err.Error()}
	}

	go shellPlugin.Execute(ser)

	return &utils.Response{Code: code.Success}
}

type ExecShellPlugin struct {
	*BasePlugin
	Params *serializers.ExecShellSerializer
	Result *ExecShellPluginResult
}

type ExecShellPluginResult struct {
	Env map[string]interface{} `json:"env"`
}

func NewExecShellPlugin(ser *serializers.ExecShellSerializer) (*ExecShellPlugin, error) {
	execPlugin := &ExecShellPlugin{
		BasePlugin: NewBasePlugin(ser.JobId, PluginBuildCodeToImage),
		Params:     ser,
		Result: &ExecShellPluginResult{
			Env: make(map[string]interface{}),
		},
	}
	execPlugin.Executor = execPlugin

	return execPlugin, nil
}

func (b *ExecShellPlugin) execute() (interface{}, error) {
	if b.Params.Resource.Value == "" {
		return nil, fmt.Errorf("执行脚本目标资源参数为空，请检查流水线配置")
	}
	if b.Params.Resource.Type == ResourceTypeImage {
		if err := b.execImage(); err != nil {
			return nil, err
		}
	} else if b.Params.Resource.Type == ResourceTypeHost {
		if err := b.execSsh(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (b *ExecShellPlugin) execImage() error {
	image := b.Params.Resource.Value
	shell := b.Params.Shell
	if shell == "" {
		shell = "bash"
	}
	dockerRunCmd := fmt.Sprintf("docker run --rm -i -v /pipeline:/pipeline -w /pipeline --entrypoint sh %s -c \"%s -cx '%s' 2>&1\"", image, shell, b.Params.Script)
	klog.Infof("job=%d code build cmd: %s", b.JobId, dockerRunCmd)
	cmd := exec.Command("bash", "-xc", dockerRunCmd)
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	if err := cmd.Run(); err != nil {
		klog.Errorf("job=%d build error: %v", b.JobId, err)
		return fmt.Errorf("build code error: %v", err)
	}
	return nil
}

func (b *ExecShellPlugin) execSsh() error {
	// 建立SSH客户端连接
	host := b.Params.Resource.Value
	if b.Params.Port != "" {
		host += ":" + b.Params.Port
	} else {
		host += ":22"
	}
	var auth ssh.AuthMethod
	if b.Params.Resource.Secret.Type == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(b.Params.Resource.Secret.PrivateKey))
		if err != nil {
			b.Log("parse ssh public key error: %s", err.Error())
			return err
		}
		auth = ssh.PublicKeys(signer)
	} else if b.Params.Resource.Secret.Type == "password" {
		auth = ssh.Password(b.Params.Resource.Secret.Password)
	}
	client, err := ssh.Dial("tcp", host, &ssh.ClientConfig{
		User:            b.Params.Resource.Secret.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		b.Log("ssh host %s error: %s", host, err.Error())
		return err
	}
	b.Log("连接主机%s成功", host)

	// 建立新会话
	session, err := client.NewSession()
	defer session.Close()
	if err != nil {
		b.Log("ssh host %s new session error: %s", host, err.Error())
		return err
	}
	b.Log("建立session成功，开始执行脚本")
	session.Stdout = b.Logger

	cmd := fmt.Sprintf("bash -cx '%s' 2>&1", b.Params.Script)
	err = session.Run(cmd)
	if err != nil {
		b.Log("执行脚本失败: %s", err.Error())
		return err
	}
	return nil
}
