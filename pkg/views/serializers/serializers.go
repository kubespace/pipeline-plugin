package serializers

type ImageBuildInfo struct {
	DockerfilePath string `json:"dockerfile_path"`
	ImageName      string `json:"image_name"`
}

type CodeSecret struct {
	Type string `json:"type"`
	User string `json:"user"`
	Password string `json:"password"`
	PrivateKey string `json:"private_key"`
	AccessToken string `json:"access_token"`
}

type BuildCodeToImageSerializer struct {
	JobId uint `json:"job_id"`

	CodeUrl                string `json:"code_url"`
	CodeBranch                string `json:"code_branch"`
	CodeCommitId                string `json:"code_commit_id"`
	CodeSecret *CodeSecret `json:"code_secret"`
	CodeBuildType          string `json:"code_build_type"`
	CodeBuildImage         string `json:"code_build_image"`
	CodeBuildImageAuthUser string `json:"code_build_image_auth_user"`
	CodeBuildImageAuthPwd  string `json:"code_build_image_auth_pwd"`
	CodeBuildFile          string `json:"code_build_file"`
	CodeBuildScript        string `json:"code_build_script"`
	CodeBuildExec          string `json:"code_build_exec"`

	ImageBuildServer   string           `json:"image_build_server"`
	ImageBuildAuthUser string           `json:"image_build_auth_user"`
	ImageBuildAuthPwd  string           `json:"image_build_auth_pwd"`
	ImageBuildInfos    []ImageBuildInfo `json:"image_build_infos"`
}
