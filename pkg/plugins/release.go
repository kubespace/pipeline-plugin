package plugins

import (
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	sshgit "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kubespace/pipeline-plugin/pkg/models"
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

type Releaser struct{}

func NewReleaser() *Releaser {
	return &Releaser{}
}

func (b *Releaser) Release(ser *serializers.ReleaseSerializer) *utils.Response {
	if ser.Version == "" {
		return &utils.Response{Code: code.ParamsError, Msg: "发布版本号为空"}
	}
	releasePlugin, err := NewReleaserPlugin(ser)
	if err != nil {
		return &utils.Response{Code: code.InitError, Msg: err.Error()}
	}

	go releasePlugin.Execute(ser)

	return &utils.Response{Code: code.Success}
}

type ReleaserPlugin struct {
	*BasePlugin
	Params  *serializers.ReleaseSerializer
	CodeDir string
	Result  *ReleaserPluginResult
	Images  []string
}

type ReleaserPluginResult struct {
	Version string `json:"version"`
	Images  string `json:"images"`
}

func NewReleaserPlugin(ser *serializers.ReleaseSerializer) (*ReleaserPlugin, error) {
	releaserPlugin := &ReleaserPlugin{
		BasePlugin: NewBasePlugin(ser.JobId, PluginBuildCodeToImage),
		Params:     ser,
		Result: &ReleaserPluginResult{
			Version: ser.Version,
			Images:  "",
		},
	}
	codeDir := utils.GetCodeRepoName(ser.CodeUrl)
	if codeDir == "" {
		klog.Errorf("job=%d get empty code repo name", ser.JobId)
		return nil, fmt.Errorf("get empty code repo name")
	}
	absCodeDir, _ := filepath.Abs(releaserPlugin.RootDir + "/" + codeDir)
	releaserPlugin.CodeDir = absCodeDir
	releaserPlugin.Executor = releaserPlugin

	return releaserPlugin, nil
}

func (r *ReleaserPlugin) execute() (interface{}, error) {
	err := models.Models.PipelineReleaseManager.Add(r.Params.WorkspaceId, r.Params.Version, r.JobId)
	if err != nil {
		return nil, err
	}
	if r.Params.CodeUrl != "" {
		if err = r.clone(); err != nil {
			return nil, err
		}
	}
	if r.Params.Images != "" {
		if err = r.tagImage(); err != nil {
			return nil, err
		}
		r.Result.Images = strings.Join(r.Images, ",")
	}
	return r.Result, nil
}

func (r *ReleaserPlugin) clone() error {
	os.RemoveAll(r.CodeDir)
	r.Log("git clone %v", r.Params.CodeUrl)
	time.Sleep(1)
	var auth transport.AuthMethod
	var err error
	if r.Params.CodeSecret.Type == "key" {
		privateKey, err := sshgit.NewPublicKeys("git", []byte(r.Params.CodeSecret.PrivateKey), "")
		if err != nil {
			return fmt.Errorf("生成代码密钥失败：" + err.Error())
		}
		privateKey.HostKeyCallbackHelper = sshgit.HostKeyCallbackHelper{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		auth = privateKey
	} else if r.Params.CodeSecret.Type == "password" {
		auth = &http.BasicAuth{
			Username: r.Params.CodeSecret.User,
			Password: r.Params.CodeSecret.Password,
		}
	}
	repo, err := git.PlainClone(r.CodeDir, false, &git.CloneOptions{
		Auth:     auth,
		URL:      r.Params.CodeUrl,
		Progress: r.Logger,
	})
	if err != nil {
		r.Log("克隆代码仓库失败：%v", err)
		klog.Errorf("job=%d clone %s error: %v", r.BasePlugin.JobId, r.Params.CodeUrl, err)
		return fmt.Errorf("git clone %s error: %v", r.Params.CodeUrl, err)
	}
	w, err := repo.Worktree()
	if err != nil {
		r.Log("克隆代码仓库失败：%v", err)
		klog.Errorf("job=%d clone %s error: %v", r.BasePlugin.JobId, r.Params.CodeUrl, err)
		return fmt.Errorf("git clone %s error: %v", r.Params.CodeUrl, err)
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(r.Params.CodeCommitId),
	})
	if err != nil {
		r.Log("git checkout %s 失败：%v", r.Params.CodeCommitId, err)
		klog.Errorf("job=%d git checkout %s error: %v", r.BasePlugin.JobId, r.Params.CodeCommitId, err)
		return fmt.Errorf("git checkout %s error: %v", r.Params.CodeCommitId, err)
	}
	r.Log("git tag %s", r.Params.Version)
	_, err = repo.CreateTag(r.Params.Version, plumbing.NewHash(r.Params.CodeCommitId), &git.CreateTagOptions{
		Message: r.Params.Version,
		Tagger: &object.Signature{
			Name:  "kubespace",
			Email: "kubespace@kubespace.cn",
			When:  time.Now(),
		},
	})
	if err != nil {
		r.Log("git tag error: %s", err.Error())
		if !errors.Is(err, git.ErrTagExists) {
			return fmt.Errorf("git tag error: %s", err.Error())
		}
	}
	po := &git.PushOptions{
		RemoteName: "origin",
		Progress:   r.Logger,
		RefSpecs:   []config.RefSpec{config.RefSpec("refs/tags/*:refs/tags/*")},
		Auth:       auth,
	}
	r.Log("git push --tags")
	err = repo.Push(po)
	if err != nil {
		r.Log("git push error: %s", err.Error())
		return err
	}

	return nil
}

func (r *ReleaserPlugin) tagImage() error {
	images := strings.Split(r.Params.Images, ",")
	if r.Params.ImageBuildRegistry.User != "" && r.Params.ImageBuildRegistry.Password != "" {
		if err := r.loginDocker(r.Params.ImageBuildRegistry.User, r.Params.ImageBuildRegistry.Password, r.Params.ImageBuildRegistry.Registry); err != nil {
			r.Log("docker login %s error: %v", r.Params.ImageBuildRegistry.Registry, err)
			klog.Errorf("docker login %s error: %v", r.Params.ImageBuildRegistry.Registry, err)
		}
	}
	for _, image := range images {
		if err := r.tagAndPushImage(image); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReleaserPlugin) loginDocker(user string, password string, server string) error {
	r.Log("docker login %s", server)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("docker login -u %s -p %s %s", user, password, server))
	cmd.Stdout = r.Logger
	cmd.Stderr = r.Logger
	return cmd.Run()
}

func (r *ReleaserPlugin) tagAndPushImage(image string) error {
	dockerBuildCmd := fmt.Sprintf("docker pull %s", image)
	cmd := exec.Command("bash", "-xc", dockerBuildCmd)
	cmd.Stdout = r.Logger
	cmd.Stderr = r.Logger
	if err := cmd.Run(); err != nil {
		r.Log("拉取镜像%s错误：%v", image, err)
		klog.Errorf("pull image error: %v", err)
		return fmt.Errorf("拉取镜像%s错误：%v", image, err)
	}
	newImage := strings.Split(image, ":")[0] + ":" + r.Params.Version
	cmd = exec.Command("bash", "-xc", "docker tag "+image+" "+newImage)
	cmd.Stdout = r.Logger
	cmd.Stderr = r.Logger
	if err := cmd.Run(); err != nil {
		r.Log("镜像打标签%s错误：%v", image, err)
		klog.Errorf("tag image error: %v", err)
		return fmt.Errorf("镜像打标签%s错误：%v", image, err)
	}
	if err := r.pushImage(newImage); err != nil {
		return err
	}
	r.Images = append(r.Images, newImage)
	rmiImage := fmt.Sprintf("docker rmi %s && docker rmi %s", image, newImage)
	cmd = exec.Command("bash", "-xc", rmiImage)
	cmd.Stdout = r.Logger
	cmd.Stderr = r.Logger
	if err := cmd.Run(); err != nil {
		r.Log("删除本地镜像%s错误：%v", image, err)
		klog.Errorf("rmi image error: %v", err)
		return fmt.Errorf("删除本地构建镜像%s错误：%v", image, err)
	}
	return nil
}

func (r *ReleaserPlugin) pushImage(imageUrl string) error {
	pushCmd := fmt.Sprintf("docker push %s", imageUrl)
	cmd := exec.Command("bash", "-xc", pushCmd)
	cmd.Stdout = r.Logger
	cmd.Stderr = r.Logger
	if err := cmd.Run(); err != nil {
		r.Log("docker push %s：%v", imageUrl, err)
		klog.Errorf("push image error: %v", err)
		return fmt.Errorf("推送镜像%s错误：%v", imageUrl, err)
	}
	return nil
}
