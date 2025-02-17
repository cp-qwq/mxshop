package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"mxshop_api/user_web/global"
	"mxshop_api/user_web/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	AccessKey  []byte
	RefreshKey []byte
}

var (
	ErrTokenExpired     = errors.New("token已过期")
	ErrTokenInvalid     = errors.New("无效token")
	ErrTokenMalformed   = errors.New("非法token格式")
	ErrTokenWrongIssuer = errors.New("签发方不匹配")
)

func NewJWT() *JWT {
	return &JWT{
		AccessKey:  []byte(global.ServerConfig.JWTInfo.AccessKey),
		RefreshKey: []byte(global.ServerConfig.JWTInfo.RefreshKey),
	}
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "请提供访问令牌"})
			return
		}

		j := NewJWT()
		claims, err := j.ParseAccessToken(tokenString)
		if err != nil {
			handleTokenError(c, err)
			return
		}

		if claims.Issuer != global.ServerConfig.JWTInfo.Issuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40104, "msg": "签发方不匹配"})
			return
		}

		c.Set("userId", claims.ID)
		c.Set("claims", claims)
		c.Next()
	}
}

// 生成双令牌
func (j *JWT) GenerateTokenPair(claims models.AccessClaims) (*models.TokenPair, error) {
	// Access Token
	accessToken, err := j.CreateAccessToken(claims)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	// Refresh Token
	refreshToken, err := j.CreateRefreshToken(models.RefreshClaims{
		UserID: claims.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(global.ServerConfig.JWTInfo.RefreshExpire)),
			Issuer:    global.ServerConfig.JWTInfo.Issuer,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(global.ServerConfig.JWTInfo.AccessExpire.Seconds()),
	}, nil
}

// 创建 AccessToken
func (j *JWT) CreateAccessToken(claims models.AccessClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.AccessKey)
}

// 创建 RefreshToken
func (j *JWT) CreateRefreshToken(claims models.RefreshClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.RefreshKey)
}

// 解析Access Token
func (j *JWT) ParseAccessToken(tokenString string) (*models.AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期签名方法: %v", token.Header["alg"])
		}
		return j.AccessKey, nil
	})

	if err != nil {
		return handleValidationError(err)
	}

	if claims, ok := token.Claims.(*models.AccessClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrTokenInvalid
}

// 解析Refresh Token
func (j *JWT) ParseRefreshToken(tokenString string) (*models.RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期签名方法: %v", token.Header["alg"])
		}
		return j.RefreshKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*models.RefreshClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrTokenInvalid
}

func extractToken(c *gin.Context) string {
	token := c.GetHeader("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		return token[7:]
	}
	return c.Query("access_token")
}

func handleValidationError(err error) (*models.AccessClaims, error) {
	// 检查 token 是否是过期的
	if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, ErrTokenExpired
	}

	// 检查 token 是否格式错误
	if errors.Is(err, jwt.ErrTokenMalformed) {
		return nil, ErrTokenMalformed
	}

	// 检查 token 是否无效
	if errors.Is(err, jwt.ErrTokenNotValidYet) {
		return nil, errors.New("令牌尚未生效")
	}

	// 检查签名是否无效
	if errors.Is(err, jwt.ErrSignatureInvalid) {
		return nil, ErrTokenInvalid
	}

	// 如果是其他类型的错误，返回通用错误
	return nil, fmt.Errorf("无法处理令牌: %v", err)
}

func handleTokenError(c *gin.Context, err error) {
	// 优化token错误的处理
	switch {
	case errors.Is(err, ErrTokenExpired):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 40101,
			"msg":  "访问令牌已过期，请使用刷新令牌续期",
		})
	case errors.Is(err, ErrTokenInvalid):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 40102,
			"msg":  "无效令牌",
		})
	case errors.Is(err, ErrTokenMalformed):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 40103,
			"msg":  "令牌格式错误",
		})
	default:
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 40100,
			"msg":  "认证失败",
		})
	}
}