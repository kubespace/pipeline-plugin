package plugins

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	sshgit "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"github.com/kubespace/pipeline-plugin/pkg/utils/code"
	"github.com/kubespace/pipeline-plugin/pkg/views/serializers"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	CodeBuildTypeNone   = "none"
	CodeBuildTypeFile   = "file"
	CodeBuildTypeScript = "script"
)

type CodeBuilder struct{}

func NewBuilder() *CodeBuilder {
	return &CodeBuilder{}
}

func (b *CodeBuilder) BuildCodeToImage(ser *serializers.BuildCodeToImageSerializer) *utils.Response {
	buildCodePlugin, err := NewCodeBuilderPlugin(ser)
	if err != nil {
		return &utils.Response{Code: code.InitError, Msg: err.Error()}
	}

	go buildCodePlugin.Execute(ser)

	return &utils.Response{Code: code.Success}
}

type CodeBuilderPlugin struct {
	*BasePlugin
	Params  *serializers.BuildCodeToImageSerializer
	CodeDir string
	Images  []string
	Result  *CodeBuilderPluginResult
}

type CodeBuilderPluginResult struct {
	ImageUrl        string `json:"images"`
	ImageRegistry   string `json:"image_registry"`
	ImageRegistryId int    `json:"image_registry_id"`
}

func NewCodeBuilderPlugin(ser *serializers.BuildCodeToImageSerializer) (*CodeBuilderPlugin, error) {
	if ser.ImageBuildRegistry.Registry == "" {
		ser.ImageBuildRegistry.Registry = "docker.io"
	}
	buildCodePlugin := &CodeBuilderPlugin{
		BasePlugin: NewBasePlugin(ser.JobId, PluginBuildCodeToImage),
		Params:     ser,
		Result: &CodeBuilderPluginResult{
			ImageUrl:        "",
			ImageRegistryId: ser.ImageBuildRegistryId,
			ImageRegistry:   ser.ImageBuildRegistry.Registry,
		},
	}
	codeDir := utils.GetCodeRepoName(ser.CodeUrl)
	if codeDir == "" {
		klog.Errorf("job=%d get empty code repo name", ser.JobId)
		return nil, fmt.Errorf("get empty code repo name")
	}
	absCodeDir, _ := filepath.Abs(buildCodePlugin.RootDir + "/" + codeDir)
	buildCodePlugin.CodeDir = absCodeDir
	buildCodePlugin.Executor = buildCodePlugin

	return buildCodePlugin, nil
}

func (b *CodeBuilderPlugin) execute() (interface{}, error) {
	if err := b.clone(); err != nil {
		return nil, err
	}
	if err := b.buildCode(); err != nil {
		return nil, err
	}
	if err := b.buildImages(); err != nil {
		return nil, err
	}
	return b.Result, nil
}

func (b *CodeBuilderPlugin) clone() error {
	os.RemoveAll(b.CodeDir)
	b.Log("git clone %v", b.Params.CodeUrl)
	time.Sleep(1)
	var auth transport.AuthMethod
	var err error
	if b.Params.CodeSecret.Type == "key" {
		privateKey, err := sshgit.NewPublicKeys("git", []byte(b.Params.CodeSecret.PrivateKey), "")
		if err != nil {
			return fmt.Errorf("生成代码密钥失败：" + err.Error())
		}
		privateKey.HostKeyCallbackHelper = sshgit.HostKeyCallbackHelper{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		auth = privateKey
	} else if b.Params.CodeSecret.Type == "password" {
		auth = &http.BasicAuth{
			Username: b.Params.CodeSecret.User,
			Password: b.Params.CodeSecret.Password,
		}
	}
	r, err := git.PlainClone(b.CodeDir, false, &git.CloneOptions{
		Auth:     auth,
		URL:      b.Params.CodeUrl,
		Progress: b.Logger,
	})
	if err != nil {
		b.Log("克隆代码仓库失败：%v", err)
		klog.Errorf("job=%d clone %s error: %v", b.BasePlugin.JobId, b.Params.CodeUrl, err)
		return fmt.Errorf("git clone %s error: %v", b.Params.CodeUrl, err)
	}
	w, err := r.Worktree()
	if err != nil {
		b.Log("克隆代码仓库失败：%v", err)
		klog.Errorf("job=%d clone %s error: %v", b.BasePlugin.JobId, b.Params.CodeUrl, err)
		return fmt.Errorf("git clone %s error: %v", b.Params.CodeUrl, err)
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(b.Params.CodeCommitId),
	})
	if err != nil {
		b.Log("git checkout %s 失败：%v", b.Params.CodeCommitId, err)
		klog.Errorf("job=%d git checkout %s error: %v", b.BasePlugin.JobId, b.Params.CodeCommitId, err)
		return fmt.Errorf("git checkout %s error: %v", b.Params.CodeCommitId, err)
	}
	return nil
}

func (b *CodeBuilderPlugin) buildCode() error {
	if b.Params.CodeBuildType == CodeBuildTypeNone || !b.Params.CodeBuild {
		b.Log("跳过代码构建")
		return nil
	}
	if b.Params.CodeBuildImage.Value == "" {
		b.Log("构建代码镜像为空，请检查流水线配置")
		return fmt.Errorf("build code image is empty")
	}
	codeBuildFile := ""
	codeBuildType := b.Params.CodeBuildType
	if codeBuildType == "" {
		codeBuildType = CodeBuildTypeScript
	}
	if b.Params.CodeBuildType == CodeBuildTypeScript {
		if b.Params.CodeBuildScript == "" {
			b.Log("代码构建脚本为空")
			return nil
		} else {
			codeBuildFile = b.CodeDir + "/.build.sh"
			f, err := os.Create(codeBuildFile)
			defer f.Close()
			if err != nil && !os.IsExist(err) {
				b.Log("创建临时脚本文件%s错误：%v", codeBuildFile, err)
				klog.Errorf("job=%d create build file error: %v", b.JobId, err)
				return fmt.Errorf("create %s file error: %v", codeBuildFile, err)
			}
			if _, err = f.Write([]byte(b.Params.CodeBuildScript)); err != nil {
				b.Log("写入临时脚本%s错误：%v", codeBuildFile, err)
				klog.Errorf("job=%d write build error: %v", b.JobId, err)
				return fmt.Errorf("write build file error: %v", err)
			}
			codeBuildFile = ".build.sh"
		}
	} else if b.Params.CodeBuildType == CodeBuildTypeFile {
		codeBuildFile = b.Params.CodeBuildFile
	}
	if codeBuildFile == "" {
		b.Log("代码构建脚本文件为空")
		return nil
	}
	shExec := b.Params.CodeBuildExec
	if shExec == "" {
		shExec = "bash"
	}

	dockerRunCmd := fmt.Sprintf("docker run --net=host --rm -i -v %s:/app -w /app --entrypoint sh %s -c \"%s -ex /app/%s 2>&1\"", b.CodeDir, b.Params.CodeBuildImage.Value, shExec, codeBuildFile)
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

func (b *CodeBuilderPlugin) buildImages() error {
	timeStr := fmt.Sprintf("%d", time.Now().Unix())
	for _, buildImage := range b.Params.ImageBuilds {
		imageName := buildImage.Image
		if imageName == "" {
			b.Log("not found build image parameter")
			return fmt.Errorf("not found build image parameter")
		}
		imageName = strings.Split(imageName, ":")[0]
		if b.Params.ImageBuildRegistry.Registry != "" {
			imageName = b.Params.ImageBuildRegistry.Registry + "/" + imageName + ":" + timeStr
		} else {
			imageName = "docker.io/" + imageName + ":" + timeStr
		}
		dockerfile := buildImage.Dockerfile
		if dockerfile == "" {
			dockerfile = "Dockerfile"
		}
		if err := b.buildAndPushImage(dockerfile, imageName); err != nil {
			return err
		}
	}
	b.Result.ImageUrl = strings.Join(b.Images, ",")
	return nil
}

func (b *CodeBuilderPlugin) buildAndPushImage(dockerfilePath string, imageName string) error {
	dockerfile := b.CodeDir + "/" + dockerfilePath
	baseDockerfile := filepath.Dir(dockerfile)
	dockerBuildCmd := fmt.Sprintf("docker build -t %s -f %s %s", imageName, dockerfile, baseDockerfile)
	cmd := exec.Command("bash", "-xc", dockerBuildCmd)
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	if err := cmd.Run(); err != nil {
		b.Log("构建镜像%s错误：%v", imageName, err)
		klog.Errorf("build image error: %v", err)
		return fmt.Errorf("构建镜像%s错误：%v", imageName, err)
	}
	if err := b.pushImage(imageName); err != nil {
		return err
	}
	b.Images = append(b.Images, imageName)
	cmd = exec.Command("bash", "-xc", "docker rmi "+imageName)
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	if err := cmd.Run(); err != nil {
		b.Log("删除本地镜像%s错误：%v", imageName, err)
		klog.Errorf("remove image %s error: %v", imageName, err)
		//return fmt.Errorf("删除本地构建镜像%s错误：%v", imageName, err)
	}
	return nil
}

func (b *CodeBuilderPlugin) loginDocker(user string, password string, server string) error {
	cmd := exec.Command("docker", fmt.Sprintf("login -u %s -p %s %s", user, password, server))
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	return cmd.Run()
}

func (b *CodeBuilderPlugin) pushImage(imageUrl string) error {
	if b.Params.ImageBuildRegistry.User != "" && b.Params.ImageBuildRegistry.Password != "" {
		if err := b.loginDocker(b.Params.ImageBuildRegistry.User, b.Params.ImageBuildRegistry.Password, b.Params.ImageBuildRegistry.Registry); err != nil {
			b.Log("docker login %s error: %v", b.Params.ImageBuildRegistry.Registry, err)
			klog.Errorf("docker login %s error: %v", b.Params.ImageBuildRegistry.Registry, err)
		}
	}
	pushCmd := fmt.Sprintf("docker push %s", imageUrl)
	cmd := exec.Command("bash", "-xc", pushCmd)
	cmd.Stdout = b.Logger
	cmd.Stderr = b.Logger
	if err := cmd.Run(); err != nil {
		b.Log("docker push %s：%v", imageUrl, err)
		klog.Errorf("push image error: %v", err)
		return fmt.Errorf("推送镜像%s错误：%v", imageUrl, err)
	}
	return nil
}
