package main

import (
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/inner/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"mxshop_api/goods_web/global"
	"mxshop_api/goods_web/initialize"
	"mxshop_api/goods_web/utils"
	"mxshop_api/goods_web/utils/register/consul"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 1. 初始化 Logger
	initialize.InitLogger()

	//2. 初始化配置文件
	initialize.InitConfig()

	//3. 初始化routers
	router := initialize.Routers()

	//4. 初始化翻译
	if err := initialize.InitTrans("zh"); err != nil {
		panic(err)
	}

	//5. 初始化srv的连接
	initialize.InitSrvConn()

	viper.AutomaticEnv()
	//如果是本地开发环境端口号固定，线上环境启动获取端口号
	debug := viper.GetBool("DEV_CONFIG")
	if !debug {
		port, err := utils.GetFreePort()
		if err == nil {
			global.ServerConfig.Port = port
		}
	}
	registerClient := consul.NewRegistryClient(global.ServerConfig.ConsulInfo.Host, global.ServerConfig.ConsulInfo.Port)
	uuidRsp, _ := uuid.NewV4()
	serviceId := fmt.Sprintf("%s", uuidRsp)
	err := registerClient.Register(global.ServerConfig.Host, global.ServerConfig.Port, global.ServerConfig.Name, global.ServerConfig.Tags, serviceId)
	if err != nil {
		zap.S().Panic("服务注册失败:", err.Error())
	}

	zap.S().Debugf("启动服务器, 端口： %d", global.ServerConfig.Port)
	go func() {
		if err := router.Run(fmt.Sprintf(":%d", global.ServerConfig.Port)); err != nil {
			zap.S().Panic("启动失败:", err.Error())
		}
	}()
	//接收终止信号
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if err = registerClient.DeRegister(serviceId); err != nil {
		zap.S().Info("注销失败:", err.Error())
	} else {
		zap.S().Info("注销成功:")
	}
}
