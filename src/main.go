package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/twinj/uuid"
	"strconv"
	"strings"
	"time"
	"fmt"
	"database/sql"
	"net/http"
	"github.com/go-redis/redis"
	"github.com/dgrijalva/jwt-go"
	_ "github.com/go-sql-driver/mysql"
)

var (
	dbUser = "root"
	dbPass = "bigsister"
	dbHost = "10.51.0.11"
	dbPort = "3306"
	dbName = "42seoul_db"
	redis_ctx = context.Background()
)

type Handler struct {
	dbHandler		*sql.DB
	redisClient		*redis.Client
}

func InitRedisClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return client
}


func InitDB() *sql.DB {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName))
	if err != nil {
		fmt.Println("db error: ", err)
	}
	db.SetMaxIdleConns(16)
	db.SetMaxOpenConns(16)
	db.SetConnMaxLifetime(5*time.Minute)
	return db
}


func InitHandler(db *sql.DB, redis *redis.Client) *Handler {
	return &Handler{
		dbHandler: db,
		redisClient: redis,
	}
}

type User struct {
	ID uint64		`json:"id"`
	Username string	`json:"username" binding:"required"`
	Password string	`json:"password" binding:"required"`
}

type TokenData struct {
	AccessToken		string
	RefreshToken	string
	ATUuid			string
	RTUuid			string
	ATExpires		int64
	RTExpires		int64

}
//demo///////////////
var dbUserData = User{
	ID: 1,
	Username: "jai",
	Password: "qwerty",
}
/////////////////////////

type RedisHandler struct {
	RedisClient *redis.Client
}

//func (h *Handler) signUp {
//	defer h.dbHandler.Close()
//	// query DB for user and password
//	_ = h.dbHandler.QueryRow("FROM user_table SELECT user")
//
//}

func (h *RedisHandler) SaveLoginData(userId uint64, td *TokenData) error {
	at := time.Unix(td.ATExpires, 0) //converting Unix to UTC(to Time object)
	rt := time.Unix(td.RTExpires, 0)
	now := time.Now()

	errAccess := h.RedisClient.Set(redis_ctx, td.ATUuid, strconv.Itoa(int(userId)), at.Sub(now)).Err()
	if errAccess != nil {
		return errAccess
	}
	errRefresh := h.RedisClient.Set(redis_ctx, td.RTUuid, strconv.Itoa(int(userId)), rt.Sub(now)).Err()
	if errRefresh != nil {
		return errRefresh
	}
	return nil
}


func GenerateToken(userId uint64) (*TokenData, error) {
	td := &TokenData{}
	var err error

	td.ATExpires = time.Now().Add(time.Minute * 6000).Unix()
	td.RTExpires = time.Now().Add(time.Hour * 24).Unix()

	td.ATUuid = uuid.NewV4().String()
	td.RTUuid = uuid.NewV4().String()

	//generate access jwtoken
	ATClaims := jwt.MapClaims{}
	ATClaims["authorized"] = true
	ATClaims["access_uuid"] = td.ATUuid
	ATClaims["user_id"] = userId
	ATClaims["exp"] = td.ATExpires
	AT := jwt.NewWithClaims(jwt.SigningMethodHS256, ATClaims)
	td.AccessToken, err = AT.SignedString([]byte("access_secret_key")) //secret_key must be hidden!!!
	if err != nil {
		return nil, err
	}

	//generate refresh jwtoken
	RTClaims := jwt.MapClaims{}
	RTClaims["authorized"] = true
	RTClaims["refresh_uuid"] = td.RTUuid
	RTClaims["user_id"] = userId
	RTClaims["exp"] = td.RTExpires
	RT := jwt.NewWithClaims(jwt.SigningMethodHS256, ATClaims)
	td.RefreshToken, err = RT.SignedString([]byte("refresh_secret_key")) //secret_key must be hidden!!!
	if err != nil {
		return nil, err
	}

	return td, nil
}


func (h *RedisHandler) signIn(ctx *gin.Context) {
	var inputUser User

	if err := ctx.ShouldBind(&inputUser); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, "Invalid login data")
		return
	}

	if dbUserData.Username != inputUser.Username || dbUserData.Password != inputUser.Password {
		ctx.JSON(http.StatusUnprocessableEntity, "Check your username and password again!")
		return
	}
	token_data, err := GenerateToken(dbUserData.ID)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, err.Error())
		return
	}
	saveError := h.SaveLoginData(dbUserData.ID, token_data)
	if saveError != nil {
		ctx.JSON(http.StatusUnprocessableEntity, saveError.Error())
	}
	tokens := map[string]string{
		"access_token": token_data.AccessToken,
		"refresh_token": token_data.RefreshToken,
	}
	authToken := []string{"Bearer", tokens["access_token"]}

	ctx.SetCookie("Authorization",  strings.Join(authToken, " "), 100, "/", "localhost", false,  true)
	ctx.JSON(http.StatusOK, tokens)
}


type SessionData struct {
	AccessUuid string
	UserId   uint64
}


func ExtractToken(req *http.Request) string {
	bearToken := req.Header.Get("Authorization")
	strArr := strings.Split(bearToken, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}


func VerifyToken(req *http.Request) (*jwt.Token, error) {
	tokenString := ExtractToken(req)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte("access_secret_key"), nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}


func ExtractTokenMetadata(req *http.Request) (*SessionData, error) {
	token, err := VerifyToken(req)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		fmt.Print(claims)
		accessUuid, ok := claims["access_uuid"].(string)
		if !ok {
			return nil, err
		}
		userId, err := strconv.ParseUint(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
		if err != nil {
			return nil, err
		}
		return &SessionData{
			AccessUuid: accessUuid,
			UserId:   userId,
		}, nil
	}
	return nil, err
}


func (h *RedisHandler) FetchAuth(sessD *SessionData) (uint64, error) {
	userid, err := h.RedisClient.Get(redis_ctx, sessD.AccessUuid).Result()
	if err != nil {
		return 0, err
	}
	userID, _ := strconv.ParseUint(userid, 10, 64)
	return userID, nil
}


type QRAuthData struct {
	Message string `json:"message"`
}


func (h *RedisHandler) verifyQRcode(ctx *gin.Context) {
	var qr_auth QRAuthData
	if err := ctx.ShouldBind(&qr_auth); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, "invalid json'")
		return
	}
	tokenAuth, err := ExtractTokenMetadata(ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}
	fmt.Print("\n", tokenAuth)
	userId, err := h.FetchAuth(tokenAuth)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}
	userId++
	ctx.JSON(http.StatusCreated, qr_auth)
}


func isTokenValid(req *http.Request) error {
	token, err := VerifyToken(req)
	if err != nil {
		fmt.Print("VerifyToken()")
		return err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		fmt.Print("Claiming")
		return err
	}
	return nil
}


func JWTMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := isTokenValid(ctx.Request)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, err.Error())
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func loginPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK,  "login.html", nil)
}

func LoginSuccess(ctx *gin.Context) {
	ctx.HTML(http.StatusOK,  "home.html", nil)
}


func (h *RedisHandler) RegisterEndpoints(v1 *gin.RouterGroup) {
	guestUsers := v1.Group("/users")
	//guestUsers.POST("/signUp", h.signUp)
	guestUsers.POST("/signIn", h.signIn)
	authRoot := v1.Group("/qr_session")
	authRoot.Use(JWTMiddleware())
	authRoot.POST("", h.verifyQRcode)

}


func main() {
	// Init webserver
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	//router.Use(middleware.SetHeader)
	router.GET("/login", loginPage)
	v1 :=  router.Group("/api")
	// init db & redis
	//db := InitDB()
	//conn1, err := db.Query("SELECT * FROM users") // (1)
	//fmt.Println("hello?")
	//if err != nil {
	//	fmt.Println("failed...")
	//	log.Fatal(err)
	//}
	rs := InitRedisClient()
	handler := RedisHandler{RedisClient: rs}
	//handler := InitHandler(db, rs)
	//handler.RegisterEndpoints(v1)

	handler.RegisterEndpoints(v1)
	//v1 := server.Group("/api", )
	//handler.RegisterEndpoints(v1)
	router.Run(":4242")
}
