package router

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kubespace/pipeline-plugin/pkg/models/mysql"
	"github.com/kubespace/pipeline-plugin/pkg/utils"
	"github.com/kubespace/pipeline-plugin/pkg/utils/code"
	"github.com/kubespace/pipeline-plugin/pkg/views"
	"k8s.io/klog"
	"runtime"
)

type Config struct {
	MysqlOptions *mysql.Options
}

type Router struct {
	*gin.Engine
}

func NewRouter(config *Config) (*Router, error) {
	engine := gin.Default()

	engine.Use(LocalMiddleware())

	apiGroup := engine.Group("/api/v1")
	viewsets := NewViewSets()
	for group, vs := range *viewsets {
		g := apiGroup.Group(group)
		for _, v := range vs {
			g.Handle(v.Method, v.Path, apiWrapper(v.Handler))
		}
	}

	return &Router{Engine: engine}, nil
}

func apiWrapper(handler views.ViewHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		context := &views.Context{Context: c}
		res := handler(context)
		c.JSON(200, res)
	}
}

func LocalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				klog.Error("error: ", err)
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				klog.Errorf("==> %s\n", string(buf[:n]))
				msg := fmt.Sprintf("%s", err)
				resp := &utils.Response{Code: code.UnknownError, Msg: msg}
				c.JSON(200, resp)
			}
		}()
		c.Next()
	}
}
