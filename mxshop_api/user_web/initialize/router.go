package initialize

import (
	"mxshop_api/user_web/middlewares"
	"mxshop_api/user_web/router"

	"github.com/gin-gonic/gin"
)

func Routers() *gin.Engine {
	Router := gin.Default()

	// 健康检查
	Router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	// 处理跨域
	Router.Use(middlewares.Cors())
	ApiGroup := Router.Group("u/v1")

	router.InitUserRouter(ApiGroup)
	router.InitBaseRouter(ApiGroup)
	return Router
}
