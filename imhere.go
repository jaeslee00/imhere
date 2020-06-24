package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/twinj/uuid"
	"log"
	"net/http"
	"regexp"
	"time"
	_ "github.com/go-sql-driver/mysql"
	"github.com/skip2/go-qrcode"
)

type DataHandler struct {
	HashUrlHandler *redis.Client
	UserHandler *sql.DB
}

type HashCodeData struct {
	Uuid		string		`json:"uuid"`
	Code		string		`json:"hash_url"`
	ExpiresAt	int64		`json:"expires_at"`
}

type UserData struct {
	Id			uint64		`json:"id"`
	password	string		`json:"password"`
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
)

func (h *DataHandler) verifyQRcode(hashCode string, ctx *gin.Context) bool {
	//req.Header.Get("X-Forwarded-For") // capitalisation
	//req.Header.Get("x-forwarded-for") // doesn't
	//req.Header.Get("X-FORWARDED-FOR") // matter
	fmt.Print("Hello!\n")
	ip := ctx.ClientIP()
	fmt.Printf("%s\n", ip)
	return false
}

func (h *DataHandler) qrcodeVerifyHandler(ctx *gin.Context) {
	rx, _ := regexp.Compile(`^([\w@#$&%^&*\-]+)`)
	hashURL := ctx.Param("regex")
	if rx.MatchString(hashURL) == true {
		if h.verifyQRcode(hashURL, ctx) == true { // change this line to compare hash from db
			ctx.JSON(http.StatusOK, gin.H{"message": "true"})
		} else {
			ctx.JSON(http.StatusForbidden, gin.H{"match": "not authorized endpoint!"})
		}
	} else {
		ctx.JSON(http.StatusNotFound, gin.H{"message":"false"})
	}
}


func (h *DataHandler) generateHashCode() *HashCodeData {
	//with uid, time, salt generate hash url
	hashURL := &HashCodeData{
		Uuid: uuid.NewV4().String(),
		Code: "abcde",
		ExpiresAt: time.Now().Add(time.Minute * 2).Unix(),
	}
	return hashURL
}

func (h *DataHandler) saveHashUrl(hashUrl *HashCodeData, sourceIp string) error {
	currTime := time.Now()
	urlExpiresAt := time.Unix(hashUrl.ExpiresAt, 0)

	redisError := h.HashUrlHandler.Set("hash_url", hashUrl.Code, currTime.Sub(urlExpiresAt)).Err()
	if redisError != nil {
		return redisError
	}
	return nil
}

func (h *DataHandler) checkLogin(ctx *gin.Context) bool {
	if err := ctx.Request.ParseForm(); err != nil {
		log.Fatal("at checkLogin()", err)
	}
	username := ctx.Request.PostFormValue("username")
	password := ctx.Request.PostFormValue("password")
	// need to hash the password and save it in db.
	if username != "jai" && password != "qwerty" {
		return true
	}
	return false
}

func (h *DataHandler) qrcodeGenHandler(ctx *gin.Context) {
	//verify login by user
	isValidLogin := h.checkLogin(ctx)
	if isValidLogin == true {
		ctx.JSON(http.StatusForbidden, gin.H{
			"message": "wrong id or password",
		})
	} else {
		hashCode := h.generateHashCode()
		verificationURL := "localhost:4433/qrcode/verification/" + hashCode.Code
		tmpImg, err := qrcode.Encode(verificationURL, qrcode.Medium, 512)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "qrcode Generation failed"})
		}

		qrImg := base64.StdEncoding.EncodeToString(tmpImg)
		// err := h.saveHashUrl(hashURL, ctx.Request.Header.Get("X-FORWARDED-FOR")) //save to redis, so we can use it later for qrcode picture verification
		ctx.HTML(http.StatusOK, "qrcode.html", gin.H{
			"img": qrImg,
		})
	}
}

func (h *DataHandler) loginPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "login.html", nil)
}

func initDatabase() *sql.DB {
	//db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName))
	//if err != nil {
	//	fmt.Println("db error: ", err)
	//}
	//db.SetMaxIdleConns(16)
	//db.SetMaxOpenConns(16)
	//db.SetConnMaxLifetime(5*time.Minute)
	//return db
	return nil
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
	router.LoadHTMLFiles("static/qrcode.html", "static/login.html")
	//sign-in & sign-out not implemented yet
	//router.GET("/intern/sign_up")
	//router.POST("/intern/sign_up")
	//router.GET("/intern/sign_in")
	//router.POST("/intern/sign_in")
	//router.Use(CheckAuthToken())
	router.GET("/login", h.loginPage)
	//router.POST("/login", h.loginHandler)
	router.POST("/qrcode", h.qrcodeGenHandler)
	router.GET("/qrcode/verification/:regex", h.qrcodeVerifyHandler)
	router.Run(":4433")
}