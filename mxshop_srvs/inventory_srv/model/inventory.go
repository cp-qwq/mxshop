package model

type Inventory struct {
	BaseModel
	Goods   int32 `gorm:"type:int;index"` // 商品id
	Stocks  int32 `gorm:"type:int"`       // 库存
	Version int32 `gorm:"type:int"`       //分布式锁的乐观锁
}