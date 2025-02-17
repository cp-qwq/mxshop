package router

import (
	"github.com/gin-gonic/gin"
	"mxshop_api/order_web/api/shop_cart"
	"mxshop_api/order_web/middlewares"
)

func InitShopCartRouter(router *gin.RouterGroup) {
	ShopCartRouter := router.Group("shopcarts").Use(middlewares.JWTAuth())
	{
		ShopCartRouter.GET("", shop_cart.List)                 //获取购物车列表
		ShopCartRouter.POST("", shop_cart.CreateCarItem)       //加入购物车
		ShopCartRouter.PATCH("/:id", shop_cart.UpdateCarItem)  //更新购物车
		ShopCartRouter.DELETE("/:id", shop_cart.DeleteCarItem) //移除购物车
	}
}