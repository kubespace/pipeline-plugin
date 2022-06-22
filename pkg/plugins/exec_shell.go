package plugins

import (
	"bytes"
	"fmt"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"github.com/kubespace/pipeline-plugin/pkg/utils/code"
	"github.com/kubespace/pipeline-plugin/pkg/views/serializers"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	"os"
	"os/exec"
	"strings"
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
	Result map[string]interface{} `json:"env"`
}

func NewExecShellPlugin(ser *serializers.ExecShellSerializer) (*ExecShellPlugin, error) {
	execPlugin := &ExecShellPlugin{
		BasePlugin: NewBasePlugin(ser.JobId, PluginBuildCodeToImage),
		Params:     ser,
		Result:     make(map[string]interface{}),
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
	return b.Result, nil
}

func (b *ExecShellPlugin) execImage() error {
	image := b.Params.Resource.Value
	shell := b.Params.Shell
	if shell == "" {
		shell = "bash"
	}
	scriptFile := b.RootDir + "/.script.sh"
	f, err := os.Create(scriptFile)
	if err != nil {
		b.Log("create script file error: %s", err.Error())
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(b.Params.Script)); err != nil {
		b.Log("写入脚本错误：%v", err)
		klog.Errorf("job=%d write build error: %v", b.JobId, err)
		return err
	}
	scriptFileName := ".script.sh"
	var envs []string
	for name, val := range b.Params.Env {
		envs = append(envs, fmt.Sprintf("%s='%v'", name, val))
	}
	envs = append(envs, fmt.Sprintf("WORKDIR='/pipeline'"))
	env := strings.Join(envs, " ")
	dockerRunCmd := fmt.Sprintf("docker run --net=host --rm -i -v %s:/pipeline -w /pipeline --entrypoint sh %s -c \"%s %s -x %s 2>&1\"", b.RootDir, image, env, shell, scriptFileName)
	klog.Infof("job=%d code build cmd: %s", b.JobId, dockerRunCmd)
	cmd := exec.Command("bash", "-c", dockerRunCmd)
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	if err := cmd.Run(); err != nil {
		klog.Errorf("job=%d build error: %v", b.JobId, err)
		return fmt.Errorf("build code error: %v", err)
	} else {
		outputBytes, err := os.ReadFile(b.RootDir + "/output")
		if err != nil {
			if !os.IsNotExist(err) {
				b.Log("read output error: %s", err.Error())
				return err
			}
		} else {
			outEnvStr := string(outputBytes)
			b.Log("output:\n%s", outEnvStr)
			if outEnvStr != "" {
				for _, line := range strings.Split(outEnvStr, "\n") {
					if strings.Contains(line, "=") {
						splits := strings.SplitN(line, "=", 2)
						key := splits[0]
						value := splits[1]
						b.Result[key] = value
					}
				}
			}
		}
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
	if err != nil {
		b.Log("ssh host %s new session error: %s", host, err.Error())
		return err
	}
	defer session.Close()
	b.Log("建立session成功，开始执行脚本")
	session.Stdout = b.Logger
	var envs []string
	for name, val := range b.Params.Env {
		envs = append(envs, fmt.Sprintf("%s='%v'", name, val))
	}
	workDir := fmt.Sprintf("/tmp/kubespace/pipeline/%d", b.JobId)
	envs = append(envs, fmt.Sprintf("WORKDIR='%s'", workDir))
	env := strings.Join(envs, " ")

	output := fmt.Sprintf("%s/output", workDir)
	cmd := fmt.Sprintf("mkdir -p %s && cd %s && rm -rf %s && %s bash -cx '%s' 2>&1", workDir, workDir, output, env, b.Params.Script)
	err = session.Run(cmd)
	if err != nil {
		b.Log("执行脚本失败: %s", err.Error())
		return err
	}
	newSession, err := client.NewSession()
	if err != nil {
		b.Log("ssh host %s new session error: %s", host, err.Error())
		return err
	}
	defer newSession.Close()
	buffer := new(bytes.Buffer)
	newSession.Stdout = buffer
	cmd = fmt.Sprintf("bash -c '[[ -f %s ]] && cat %s; rm -rf %s'", output, output, workDir)
	err = newSession.Run(cmd)
	if err != nil {
		b.Log("获取脚本输出%s失败: %s", output, err.Error())
		return err
	} else {
		outEnvStr := buffer.String()
		b.Log("output:\n%s", outEnvStr)
		if outEnvStr != "" {
			for _, line := range strings.Split(outEnvStr, "\n") {
				if strings.Contains(line, "=") {
					splits := strings.SplitN(line, "=", 2)
					key := splits[0]
					value := splits[1]
					b.Result[key] = value
				}
			}
		}
	}
	return nil
}
