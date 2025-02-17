package router

import (
	"github.com/gin-gonic/gin"
	"mxshop_api/order_web/api/order"
	"mxshop_api/order_web/middlewares"
)

func InitOrderRouter(router *gin.RouterGroup) {
	OrderRouter := router.Group("order").Use(middlewares.JWTAuth())
	{
		//中间件的参数位置需要按业务需求而定
		OrderRouter.GET("", order.List)            //订单列表
		OrderRouter.GET("/:id", order.DetailOrder) //获取订单详细
		OrderRouter.POST("", order.CreatOrder)     //新建订单
		OrderRouter.PATCH("", order.UpdateOrder)   //更新订单
	}
}