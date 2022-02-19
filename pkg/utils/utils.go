package utils

import (
	"strings"
)

// GetCodeRepoName 获取代码库的项目名
// 如：https://github.com/test/testrepo.git -> testrepo
// git@github.com/test/testrepo.git -> testrepo
func GetCodeRepoName(codeUrl string) string {
	codeSplit := strings.Split(codeUrl, "/")
	codeDir := codeSplit[len(codeSplit)-1]
	codeSplit = strings.Split(codeDir, ".")
	return codeSplit[0]
}
