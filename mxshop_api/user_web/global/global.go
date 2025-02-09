package global

import (
	ut "github.com/go-playground/universal-translator"
	"mxshop_api/user_web/config"
	"mxshop_api/user_web/proto"
)

var (
	Trans        ut.Translator
	ServerConfig *config.ServerConfig = &config.ServerConfig{}
	NacosConfig *config.NacosConfig = &config.NacosConfig{}
	UserSrvClient proto.UserClient
)