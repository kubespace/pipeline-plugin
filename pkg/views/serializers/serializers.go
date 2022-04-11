package serializers

type ImageBuilds struct {
	Dockerfile string `json:"dockerfile"`
	Image      string `json:"image"`
}

type ImageRegistry struct {
	Registry string `json:"registry"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type Secret struct {
	Type        string `json:"type"`
	User        string `json:"user"`
	Password    string `json:"password"`
	PrivateKey  string `json:"private_key"`
	AccessToken string `json:"access_token"`
}

type PipelineResource struct {
	Type   string  `json:"type"`
	Value  string  `json:"value"`
	Secret *Secret `json:"secret"`
}

type BuildCodeToImageSerializer struct {
	JobId uint `json:"job_id"`

	CodeUrl         string           `json:"code_url"`
	CodeBranch      string           `json:"code_branch"`
	CodeCommitId    string           `json:"code_commit_id"`
	CodeSecret      *Secret          `json:"code_secret"`
	CodeBuild       bool             `json:"code_build"`
	CodeBuildType   string           `json:"code_build_type"`
	CodeBuildImage  PipelineResource `json:"code_build_image"`
	CodeBuildFile   string           `json:"code_build_file"`
	CodeBuildScript string           `json:"code_build_script"`
	CodeBuildExec   string           `json:"code_build_exec"`

	ImageBuildRegistryId int           `json:"image_registry_id"`
	ImageBuildRegistry   ImageRegistry `json:"image_build_registry"`
	ImageBuilds          []ImageBuilds `json:"image_builds"`
}

type ReleaseSerializer struct {
	JobId       uint `json:"job_id"`
	WorkspaceId uint `json:"workspace_id"`

	CodeUrl      string  `json:"code_url"`
	CodeBranch   string  `json:"code_branch"`
	CodeCommitId string  `json:"code_commit_id"`
	CodeSecret   *Secret `json:"code_secret"`

	ImageBuildRegistry ImageRegistry `json:"image_registry"`

	Version string `json:"version"`
	Images  string `json:"images"`
}
