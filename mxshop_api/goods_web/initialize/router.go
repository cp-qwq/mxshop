package initialize

import (
	"mxshop_api/goods_web/middlewares"
	"mxshop_api/goods_web/router"

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
	ApiGroup := Router.Group("g/v1")

	router.InitGoodsRouter(ApiGroup)    //商品
	router.InitCategoryRouter(ApiGroup) //商品分类
	router.InitBannerRouter(ApiGroup)   //商品轮播图
	router.InitBrandsRouter(ApiGroup)   //品牌，分类-品牌
	return Router
}
