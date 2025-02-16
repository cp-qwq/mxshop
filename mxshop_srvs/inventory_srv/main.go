package main

import (
	"flag"
	"fmt"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"mxshop_srvs/inventory_srv/global"
	"mxshop_srvs/inventory_srv/handler"
	"mxshop_srvs/inventory_srv/initialize"
	"mxshop_srvs/inventory_srv/proto"
	"mxshop_srvs/inventory_srv/utils"
	"mxshop_srvs/inventory_srv/utils/register/consul"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {

	//初始化
	initialize.InitLogger()
	initialize.InitConfig()
	initialize.InitDB()

	IP := flag.String("ip", "0.0.0.0", "ip地址")
	Port := flag.Int("port", 50059, "端口号") // 这个修改为0，如果我们从命令行带参数启动的话就不会为0
	flag.Parse()
	if *Port == 0 {
		*Port, _ = utils.GetFreePort()
	}
	zap.S().Info("服务启动地址：" + *IP + ":" + strconv.Itoa(*Port))

	server := grpc.NewServer()
	proto.RegisterInventoryServer(server, &handler.InventoryServer{})
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *IP, *Port))
	if err != nil {
		panic("failed to listen:" + err.Error())
	}

	//注册服务健康检查
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())

	//服务注册
	registryClient := consul.NewRegistryClient(global.ServerConfig.ConsulInfo.Host, global.ServerConfig.ConsulInfo.Port)
	id := uuid.NewV4()
	serverIdstr := fmt.Sprintf("%s", id)
	err = registryClient.Register(global.ServerConfig.Host, *Port, global.ServerConfig.Name, global.ServerConfig.Tags, serverIdstr)
	if err != nil {
		zap.S().Panic("服务注册失败", err.Error())
	}
	zap.S().Info("服务启动", *Port)

	// 开一个goroutine，否则会一直阻塞看不到退出的日志
	go func() {
		err = server.Serve(lis)
		if err != nil {
			panic("failed to start grpc:" + err.Error())
		}
	}()

	// 接收终止信号
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	err = registryClient.DeRegister(serverIdstr)
	if err != nil {
		zap.S().Info("注销失败", err)
	} else {
		zap.S().Info("注销成功")
	}
}