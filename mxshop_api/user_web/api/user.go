package api

import (
	"context"
	"fmt"
	"mxshop_api/user_web/forms"
	"mxshop_api/user_web/global"
	"mxshop_api/user_web/global/response"
	"mxshop_api/user_web/middlewares"
	"mxshop_api/user_web/models"
	"mxshop_api/user_web/proto"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleGrpcErrorToHttp(err error, c *gin.Context) {
	//将grpc的code转换成http的状态码
	if err != nil {
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{
					"msg": e.Message(),
				})
			case codes.Internal:
				c.JSON(http.StatusInternalServerError, gin.H{
					"msg:": "内部错误",
				})
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{
					"msg": "参数错误",
				})
			case codes.Unavailable:
				c.JSON(http.StatusInternalServerError, gin.H{
					"msg": "用户服务不可用",
				})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{
					"msg": e.Code(),
				})
			}
			return
		}
	}
}

func HandleValidatorError(c *gin.Context, err error) {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"msg": err.Error(),
		})
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": removeTopStruct(errs.Translate(global.Trans)),
	})
	return
}

func removeTopStruct(fileds map[string]string) map[string]string {
	rsp := map[string]string{}
	for field, err := range fileds {
		rsp[field[strings.Index(field, ".")+1:]] = err
	}
	return rsp
}

func GetUserList(ctx *gin.Context) {
	pn := ctx.DefaultQuery("pn", "0")
	pnInt, _ := strconv.Atoi(pn)
	pSize := ctx.DefaultQuery("psize", "10")
	pSizeInt, _ := strconv.Atoi(pSize)
	rsp, err := global.UserSrvClient.GetUserList(context.Background(), &proto.PageInfo{
		Pn:    uint32(pnInt),
		PSize: uint32(pSizeInt),
	})
	if err != nil {
		zap.S().Errorw("[GetUserList] 查询 【用户列表】 失败")
		HandleGrpcErrorToHttp(err, ctx)
		return
	}

	result := make([]interface{}, 0)
	for _, value := range rsp.Data {
		user := response.UserResponse{
			Id:       value.Id,
			NickName: value.NickName,
			//Birthday: time.Time(time.Unix(int64(value.BirthDay), 0)).Format("2006-01-02"),
			Birthday: response.JsonTime(time.Unix(int64(value.BirthDay), 0)),
			Gender:   value.Gender,
			Mobile:   value.Mobile,
		}
		result = append(result, user)
	}

	ctx.JSON(http.StatusOK, result)
}

func PassWordLogin(c *gin.Context) {
	passwordLoginForm := forms.PassWordLoginForm{}
	if err := c.ShouldBind(&passwordLoginForm); err != nil {
		HandleValidatorError(c, err)
		return
	}

	// 1. 验证手机号是否存在
	userRsp, err := global.UserSrvClient.GetUserByMobile(context.Background(), &proto.MobileRequest{
		Mobile: passwordLoginForm.Mobile,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.NotFound:
				c.JSON(http.StatusBadRequest, gin.H{"mobile": "用户不存在"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"msg": "登录失败"})
			}
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "系统错误"})
		return
	}

	// 2. 验证密码
	passRsp, err := global.UserSrvClient.CheckPassWord(context.Background(), &proto.PasswordCheckInfo{
		Password:          passwordLoginForm.PassWord,
		EncryptedPassword: userRsp.PassWord,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "密码验证失败"})
		return
	}

	if !passRsp.Success {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "密码错误"})
		return
	}

	// 3. 生成双Token
	j := middlewares.NewJWT()
	accessClaims := models.AccessClaims{
		ID:          uint(userRsp.Id),
		NickName:    userRsp.NickName,
		AuthorityId: uint(userRsp.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    global.ServerConfig.JWTInfo.Issuer,
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	tokenPair, err := j.GenerateTokenPair(accessClaims)
	if err != nil {
		zap.S().Errorf("生成令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "系统错误"})
		return
	}

	// 4. 存储RefreshToken到Redis
	refreshKey := fmt.Sprintf("refresh:%d", userRsp.Id)
	err = global.RedisClient.Set(context.Background(), refreshKey, tokenPair.RefreshToken, global.ServerConfig.JWTInfo.RefreshExpire).Err()
	if err != nil {
		zap.S().Errorf("存储RefreshToken失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "系统错误"})
		return
	}

	// 5. 返回响应
	c.JSON(http.StatusOK, gin.H{
		"id":            userRsp.Id,
		"nick_name":     userRsp.NickName,
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
	})
}

func RefreshToken(c *gin.Context) {
	type refreshRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "参数错误"})
		return
	}

	// 1. 验证RefreshToken有效性
	j := middlewares.NewJWT()
	refreshClaims, err := j.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40104, "msg": "无效的刷新令牌"})
		return
	}

	// 2. 检查Redis中的RefreshToken
	refreshKey := fmt.Sprintf("refresh:%d", refreshClaims.UserID)
	storedToken, err := global.RedisClient.Get(context.Background(), refreshKey).Result()
	if err == redis.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40105, "msg": "刷新令牌已过期"})
		return
	} else if err != nil {
		zap.S().Errorf("Redis查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": "系统错误"})
		return
	}

	if storedToken != req.RefreshToken {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40106, "msg": "刷新令牌无效"})
		return
	}

	// 3. 获取用户最新信息
	userRsp, err := global.UserSrvClient.GetUserById(context.Background(), &proto.IdRequest{
		Id: int32(refreshClaims.UserID),
	})
	if err != nil {
		HandleGrpcErrorToHttp(err, c)
		return
	}

	// 4. 生成新Token对
	newAccessClaims := models.AccessClaims{
		ID:          uint(userRsp.Id),
		NickName:    userRsp.NickName,
		AuthorityId: uint(userRsp.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    global.ServerConfig.JWTInfo.Issuer,
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	newTokenPair, err := j.GenerateTokenPair(newAccessClaims)
	if err != nil {
		zap.S().Errorf("生成新令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50002, "msg": "系统错误"})
		return
	}

	// 5. 更新Redis中的RefreshToken
	err = global.RedisClient.Set(context.Background(), refreshKey, newTokenPair.RefreshToken, global.ServerConfig.JWTInfo.RefreshExpire).Err()
	if err != nil {
		zap.S().Errorf("刷新令牌存储失败: %v", err)
	}

	// 6. 返回新Token对
	c.JSON(http.StatusOK, gin.H{
		"access_token":  newTokenPair.AccessToken,
		"refresh_token": newTokenPair.RefreshToken,
		"expires_in":    newTokenPair.ExpiresIn,
	})
}

func Register(c *gin.Context) {
	//用户注册
	registerForm := forms.RegisterForm{}
	if err := c.ShouldBind(&registerForm); err != nil {
		HandleValidatorError(c, err)
		return
	}

	//验证码
	value, err := global.RedisClient.Get(context.Background(), registerForm.Mobile).Result()
	if err == redis.Nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": "验证码错误",
		})
		return
	} else {
		if value != registerForm.Code {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": "验证码错误",
			})
			return
		}
	}

	user, err := global.UserSrvClient.CreateUser(context.Background(), &proto.CreateUserInfo{
		NickName: registerForm.Mobile,
		PassWord: registerForm.PassWord,
		Mobile:   registerForm.Mobile,
	})

	if err != nil {
		zap.S().Errorf("[Register] 查询 【新建用户失败】失败: %s", err.Error())
		HandleGrpcErrorToHttp(err, c)
		return
	}

	j := middlewares.NewJWT()
	claims := models.AccessClaims{
		ID:          uint(user.Id),
		NickName:    user.NickName,
		AuthorityId: uint(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			Issuer:    global.ServerConfig.JWTInfo.Issuer,
		},
	}
	tokenPair, err := j.GenerateTokenPair(claims)
	if err != nil {
		zap.S().Errorf("生成令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "系统错误"})
		return
	}

	// 4. 存储RefreshToken到Redis
	refreshKey := fmt.Sprintf("refresh:%d", user.Id)
	err = global.RedisClient.Set(context.Background(), refreshKey, tokenPair.RefreshToken, global.ServerConfig.JWTInfo.RefreshExpire).Err()
	if err != nil {
		zap.S().Errorf("存储RefreshToken失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "系统错误"})
		return
	}

	// 5. 返回响应
	c.JSON(http.StatusOK, gin.H{
		"id":            user.Id,
		"nick_name":     user.NickName,
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
	})
}