package handler

import (
	"context"
	"fmt"
	goredislib "github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"mxshop_srvs/inventory_srv/global"
	"mxshop_srvs/inventory_srv/model"
	"mxshop_srvs/inventory_srv/proto"
)

type InventoryServer struct {
	proto.UnimplementedInventoryServer
}

// SetInv 设置库存
func (*InventoryServer) SetInv(ctx context.Context, req *proto.GoodsInvInfo) (*emptypb.Empty, error) {
	//设置库存， 如果我要更新库存
	var inv model.Inventory
	global.DB.Where(&model.Inventory{Goods: req.GoodsId}).First(&inv)
	inv.Goods = req.GoodsId
	inv.Stocks = req.Num

	global.DB.Save(&inv)
	return &emptypb.Empty{}, nil
}

func (*InventoryServer) InvDetail(ctx context.Context, req *proto.GoodsInvInfo) (*proto.GoodsInvInfo, error) {
	var inv model.Inventory
	if result := global.DB.First(&inv, req.GoodsId); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "库存信息不存在")
	}
	return &proto.GoodsInvInfo{
		GoodsId: inv.Goods,
		Num:     inv.Stocks,
	}, nil
}

//
//// Sell 库存扣减
//// Mysql 悲观锁版本
//func (*InventoryServer) Sell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
//	// 扣减库存，本地事务
//	// 数据库基本的一个应用场景：数据库事务
//	// 并发情况之下 可能会出现超卖
//	tx := global.DB.Begin()
//	for _, goodInfo := range req.GoodsInfo {
//		var inv model.Inventory
//		if result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
//			tx.Rollback() // 回滚之前的操作
//			return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
//		}
//		// 判断库存是否充足
//		if inv.Stocks < goodInfo.Num {
//			tx.Rollback() // 回滚之前的操作
//			return nil, status.Errorf(codes.ResourceExhausted, "库存不足")
//		}
//		// 扣减，这里会出现数据不一致的问题
//		inv.Stocks -= goodInfo.Num
//		tx.Save(&inv) // 一旦使用了事务的，保存修改数据库的操作就需要使用事务的tx，而不能使用db
//	}
//	tx.Commit() // 需要自己手动提交操作
//	return &emptypb.Empty{}, nil
//}

// Mysql 乐观锁版本
// 利用版本号机制来实现乐观锁
//func (*InventoryServer) Sell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
//	// 扣减库存，本地事务
//	// 数据库基本的一个应用场景：数据库事务
//	// 并发情况之下 可能会出现超卖 1
//	tx := global.DB.Begin()
//	for _, goodInfo := range req.GoodsInfo {
//		var inv model.Inventory
//
//		for {
//			if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
//				tx.Rollback() // 回滚之前的操作
//				return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
//			}
//			// 判断库存是否充足
//			if inv.Stocks < goodInfo.Num {
//				tx.Rollback() // 回滚之前的操作
//				return nil, status.Errorf(codes.ResourceExhausted, "库存不足")
//			}
//			// 扣减，这里会出现数据不一致的问题
//			inv.Stocks -= goodInfo.Num
//			//update inventory set stocks = stocks-1, version=version+1 where goods=goods and version=version
//			//这种写法有瑕疵，为什么？
//			//零值 对于int类型来说 默认值是0 这种会被gorm给忽略掉
//			//解决方案：试用gorm提供是select写法强制更新指定的字段
//			if result := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where("goods = ? and version= ?",
//				goodInfo.GoodsId, inv.Version).Updates(model.Inventory{Stocks: inv.Stocks, Version: inv.Version + 1}); result.RowsAffected == 0 {
//				zap.S().Info("库存扣减失败")
//			} else {
//				break
//			}
//		}
//	}
//	tx.Commit() // 需要自己手动提交操作
//	return &emptypb.Empty{}, nil
//}

// Redis 分布式锁
func (*InventoryServer) Sell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	client := goredislib.NewClient(&goredislib.Options{
		Addr: "127.0.0.1:6379",
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	rs := redsync.New(pool)

	tx := global.DB.Begin()
	for _, goodInfo := range req.GoodsInfo {
		var inv model.Inventory
		mutex := rs.NewMutex(fmt.Sprintf("goods_%d", goodInfo.GoodsId))
		if err := mutex.Lock(); err != nil {
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚之前的操作
			return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
		}
		//判断库存是否充足
		if inv.Stocks < goodInfo.Num {
			tx.Rollback() //回滚之前的操作
			return nil, status.Errorf(codes.ResourceExhausted, "库存不足")
		}
		//扣减， 会出现数据不一致的问题 - 锁，分布式锁
		inv.Stocks -= goodInfo.Num
		tx.Save(&inv)

		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
	}
	tx.Commit() // 需要自己手动提交操作
	//m.Unlock() //释放锁
	return &emptypb.Empty{}, nil
}

// Reback 库存归还
func (*InventoryServer) Reback(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//库存归还： 1：订单超时归还 2. 订单创建失败，归还之前扣减的库存 3. 手动归还
	tx := global.DB.Begin()
	for _, goodInfo := range req.GoodsInfo {
		var inv model.Inventory
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚之前的操作
			return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
		}

		//扣减， 会出现数据不一致的问题 - 锁，分布式锁
		inv.Stocks += goodInfo.Num
		tx.Save(&inv)
	}
	tx.Commit() // 需要自己手动提交操作
	return &emptypb.Empty{}, nil
}