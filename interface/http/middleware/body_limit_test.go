package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyLimitRequestWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	maxBytes := int64(1024)
	smallBody := strings.Repeat("a", 100)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(BodyLimit(maxBytes))
	router.POST("/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "failed to read body")
			return
		}
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(smallBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, smallBody, w.Body.String())
}

func TestBodyLimitRequestExceedsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	maxBytes := int64(100)
	largeBody := strings.Repeat("a", 200)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(BodyLimit(maxBytes))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(largeBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, "body too large", w.Body.String())
}

func TestBodyLimitExactlyAtLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	maxBytes := int64(100)
	exactBody := strings.Repeat("a", 100)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(BodyLimit(maxBytes))
	router.POST("/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(exactBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, exactBody, w.Body.String())
}

func TestBodyLimitEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	maxBytes := int64(100)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(BodyLimit(maxBytes))
	router.POST("/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(nil))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestBodyLimitOneByteOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	maxBytes := int64(100)
	overBody := strings.Repeat("a", 101)

	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	router.Use(BodyLimit(maxBytes))
	router.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(overBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}
