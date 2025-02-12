package router

import (
	"github.com/gin-gonic/gin"
	"mxshop_api/goods_web/api/category"
	"mxshop_api/goods_web/middlewares"
)

func InitCategoryRouter(router *gin.RouterGroup) {
	CategoryRouter := router.Group("categorys")
	{
		CategoryRouter.GET("", category.List)                                                            //商品分类列表
		CategoryRouter.GET("/:id", category.Detail)                                                      //获取商品分类详情
		CategoryRouter.POST("", middlewares.JWTAuth(), middlewares.IsAdminAuth(), category.New)          //添加分类
		CategoryRouter.DELETE("/:id", middlewares.JWTAuth(), middlewares.IsAdminAuth(), category.Delete) //删除分类
		CategoryRouter.PUT("/:id", middlewares.JWTAuth(), middlewares.IsAdminAuth(), category.Update)    //更新分类
	}
}
