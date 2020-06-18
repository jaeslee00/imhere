package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/twinj/uuid"
	"net/http"
	"regexp"
	"time"
	_ "github.com/go-sql-driver/mysql"
)

type DataHandler struct {
	HashUrlHandler *redis.Client
	UserHandler *sql.DB
}

type HashUrlData struct {
	Uuid		string		`json:"uuid"`
	Url			string		`json:"hash_url"`
	ExpiresAt	int64		`json:"expires_at"`
}

type UserData struct {
	Id			uint64		`json:"id"`
	FirstName	string		`json:"first_name"`
	LastName	string		`json:"last_name"`
	Age			uint16		`json:"age"`
	BirthDate	int64		`json:"birth_date"`
	CreatedAt	int64		`json:"create_at"`
	UpdatedAt	int64		`json:"updated_at"`
	DeviceInfo	string		`json:"device_info"` //MAC ADDRESS
}

type Attendance struct {
	Id	int64				`json:"id"`
	UserId	int64			`json:"user_id"`
	Action	int8			`json:"action"`
	createdAt int64			`json:"created_at"`
}

//DEMO USER FOR TESTING
var demoUser = UserData{
	Id: 1,
	FirstName: "jai",
	LastName: "lee",
	Age: 29,
	BirthDate: 124565423,
	CreatedAt: 126475845,
	UpdatedAt: 234545635,
	DeviceInfo: "Iphone 11 PRO",
}

//DEMO DB AUTH FOR TESTING
var (
	GENERATED_HASHCODE = "abcde"
	redis_c = context.Background()
)

func (h *DataHandler) verifyQRcode(input string) bool {
	return true
}

func (h *DataHandler) qrcodeVerifyHandler(ctx *gin.Context) {
	rx, _ := regexp.Compile(`^([\w@#$&%^&*\-]+)`)
	hashURL := ctx.Param("regex")
	fmt.Print(hashURL)
	if rx.MatchString(hashURL) == true {
		if h.verifyQRcode(hashURL) == true { // change this line to compare hash from db
			ctx.JSON(http.StatusOK, gin.H{"message": "true"})
		} else {
			ctx.JSON(http.StatusForbidden, gin.H{"match": "not authorized endpoint!"})
		}
	} else {
		ctx.JSON(http.StatusNotFound, gin.H{"message":"false"})
	}
}

func (h *DataHandler) generateHashURL() *HashUrlData {
	//with uid, time, salt generate hash url
	hashURL := &HashUrlData{
		Uuid: uuid.NewV4().String(),
		Url: "abcde",
		ExpiresAt: time.Now().Add(time.Minute * 2).Unix(),
	}
	return hashURL
}



func (h *DataHandler) saveHashUrl(hashUrl *HashUrlData, sourceIp string) error {
	currTime := time.Now()
	urlExpiresAt := time.Unix(hashUrl.ExpiresAt, 0)

	redisError := h.HashUrlHandler.Set(redis_c, "hash_url", hashUrl.Url, currTime.Sub(urlExpiresAt)).Err()
	if redisError != nil {
		return redisError
	}
	return nil
}

func (h *DataHandler) qrcodeGenHandler(ctx *gin.Context) {
	hashURL := h.generateHashURL()
	jwt, err := ctx.Request.Cookie("Authorization")
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized Access. Please check the jwt"})
	}
	err := h.saveHashUrl(hashURL, ctx.Request.Header("X-FORWARDED-FOR")) //save to redis, so we can use it later for qrcode picture verification
	ctx.JSON(http.StatusOK, gin.H{"message": hashURL})
}

func initDatabase() *sql.DB {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName))
	if err != nil {
		fmt.Println("db error: ", err)
	}
	db.SetMaxIdleConns(16)
	db.SetMaxOpenConns(16)
	db.SetConnMaxLifetime(5*time.Minute)
	return db
}
func initRedisC() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return client
}


func main() {
	router := gin.Default()

	redis := initRedisC()
	mysql := initDatabase()
	h := DataHandler{HashUrlHandler: redis, UserHandler: mysql}

	//sign-in & sign-out not implemented yet
	//router.GET("/intern/sign_up")
	//router.POST("/intern/sign_up")
	//router.GET("/intern/sign_in")
	//router.POST("/intern/sign_in")
	//router.Use(CheckAuthToken())
	router.GET("/qrcode", h.qrcodeGenHandler)
	router.GET("/qrcode/verification/:regex", h.qrcodeVerifyHandler)
	router.Run(":4433")
}
