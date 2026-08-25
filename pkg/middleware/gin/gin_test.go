package ginsentinel_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gosentinel/internal/rule"
	ginsentinel "gosentinel/pkg/middleware/gin"
	"gosentinel/pkg/sentinel"
)

func TestMiddlewareOpensCircuitAfterServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := rule.Default()
	r.Service = "checkout"
	r.Resource = "/pay"
	r.Method = http.MethodPost
	r.QPS = 100
	r.MinRequests = 1
	r.ErrorRate = 0.01
	r.OpenTimeoutMs = 60_000

	guard := sentinel.New(sentinel.Options{
		Service: "checkout",
		Rules:   []rule.Snapshot{r},
	})
	router := gin.New()
	router.Use(ginsentinel.Middleware(guard, ginsentinel.Options{}))

	handlerCalls := 0
	router.POST("/pay", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upstream failed"})
	})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first response status = %d, want %d", first.Code, http.StatusInternalServerError)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/pay", nil))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second response status = %d, want %d; body = %s", second.Code, http.StatusServiceUnavailable, second.Body.String())
	}
	if handlerCalls != 1 {
		t.Fatalf("business handler called %d times, want 1", handlerCalls)
	}
}
