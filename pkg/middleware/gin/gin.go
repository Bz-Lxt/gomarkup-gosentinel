package ginsentinel

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gosentinel/pkg/sentinel"
)

type Options struct {
	Resource   func(*gin.Context) string
	IsError    func(*gin.Context) bool
	OnBlocked  func(*gin.Context, sentinel.Reason)
}

func Middleware(g *sentinel.Guard, opts Options) gin.HandlerFunc {
	if opts.Resource == nil {
		opts.Resource = func(c *gin.Context) string { return c.FullPath() }
	}
	if opts.IsError == nil {
		opts.IsError = func(c *gin.Context) bool { return c.Writer.Status() >= 500 }
	}
	return func(c *gin.Context) {
		res := opts.Resource(c)
		if res == "" {
			res = c.Request.URL.Path
		}
		tok := g.Entry(res, c.Request.Method)
		if tok.Blocked {
			if opts.OnBlocked != nil {
				opts.OnBlocked(c, tok.Reason)
				tok.Exit(sentinel.ResultFallback)
				return
			}
			status := http.StatusTooManyRequests
			if tok.Reason == sentinel.ReasonCircuitOpen {
				status = http.StatusServiceUnavailable
			}
			c.AbortWithStatusJSON(status, gin.H{
				"error": gin.H{"code": string(tok.Reason), "message": "protected"},
			})
			tok.Exit(sentinel.ResultFallback)
			return
		}
		defer func(status int) {
			if status >= 500 && opts.IsError(c) {
				tok.Exit(sentinel.ResultError)
				return
			}
			tok.Exit(sentinel.ResultOK)
		}(c.Writer.Status())
		c.Next()
	}
}
