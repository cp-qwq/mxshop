package initialize

import (
	"mxshop_api/order_web/middlewares"
	"mxshop_api/order_web/router"

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

	// 分发路由
	ApiGroup := Router.Group("o/v1")

	router.InitOrderRouter(ApiGroup)
	router.InitShopCartRouter(ApiGroup)
	return Router
}