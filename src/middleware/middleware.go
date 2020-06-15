package middleware

import (
	"github.com/gin-gonic/gin"
	"time"
)

func SetHeader(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*'")
	ctx.Header("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, prep-check=0, max-age=0")
	ctx.Header("Last-Modified", time.Now().String())
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "-1")

	ctx.Next()
}

