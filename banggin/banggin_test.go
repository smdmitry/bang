package banggin_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/smdmitry/bang"
	"github.com/smdmitry/bang/banggin"
)

func TestErrorMiddlewareWritesProblemJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.ErrorMiddleware())
	r.GET("/users/:id", func(c *gin.Context) {
		_ = c.Error(bang.NotFound("users.not_found").Msg("Пользователь не найден"))
		c.Abort()
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusNotFound)
	}

	var pd bang.ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if pd.Status != http.StatusNotFound {
		t.Fatalf("unexpected payload status: got %d, want %d", pd.Status, http.StatusNotFound)
	}
	if pd.Instance != "/users/42" {
		t.Fatalf("unexpected instance: got %q", pd.Instance)
	}
	if pd.ErrorType != "business" {
		t.Fatalf("unexpected error type: got %q", pd.ErrorType)
	}
}

func TestErrorMiddlewareSkipsWhenAlreadyWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.ErrorMiddleware())
	r.GET("/ok", func(c *gin.Context) {
		_ = c.Error(bang.NotFound("users.not_found"))
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusAccepted)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestBindErrConvertsValidatorErrors(t *testing.T) {
	type req struct {
		Email string `validate:"required,email"`
	}

	v := validator.New(validator.WithRequiredStructEnabled())
	raw := v.Struct(req{})
	if raw == nil {
		t.Fatal("expected validator error")
	}

	err := banggin.BindErr(raw)

	var se *bang.Error
	if !errors.As(err, &se) {
		t.Fatalf("expected *bang.Error, got %T", err)
	}
	if se.GetClass() != bang.ClassValidation {
		t.Fatalf("unexpected class: got %s", se.GetClass())
	}

	fields := se.GetValidationFields()
	if len(fields) == 0 {
		t.Fatal("expected validation fields")
	}
	if fields[0].Key == "" {
		t.Fatal("expected non-empty field key")
	}
	if fields[0].Reason == "" {
		t.Fatal("expected non-empty field reason")
	}
}

func TestBindErrKeepsBangAsIs(t *testing.T) {
	orig := bang.Validation("x").Field("name", "required", "Поле обязательно")
	converted := banggin.BindErr(orig)
	if converted != orig {
		t.Fatalf("expected original error instance")
	}
}

func TestAbortBindWorksWithMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.ErrorMiddleware())
	r.POST("/bind", func(c *gin.Context) {
		banggin.AbortBind(c, io.EOF)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bind", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}

	var pd bang.ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if pd.ErrorType != "validation" {
		t.Fatalf("unexpected error type: got %q", pd.ErrorType)
	}
	if len(pd.Errors) == 0 {
		t.Fatalf("expected validation details")
	}
}

func TestErrorMiddlewareLogsOnlyUnexpectedErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	r := gin.New()
	r.Use(banggin.ErrorMiddleware())
	r.GET("/not-found", func(c *gin.Context) {
		_ = c.Error(bang.NotFound("users.not_found"))
		c.Abort()
	})
	r.GET("/unexpected", func(c *gin.Context) {
		_ = c.Error(errors.New("boom"))
		c.Abort()
	})

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	r.ServeHTTP(w1, req1)

	if strings.Contains(buf.String(), "request failed") {
		t.Fatalf("4xx errors must not be logged, got logs: %s", buf.String())
	}

	buf.Reset()

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/unexpected", nil)
	r.ServeHTTP(w2, req2)

	logs := buf.String()
	if !strings.Contains(logs, "request failed") {
		t.Fatalf("expected 5xx error log, got: %s", logs)
	}
	if !strings.Contains(logs, "http.status=500") {
		t.Fatalf("expected status in log, got: %s", logs)
	}
}

func TestRecoveryMiddlewareWritesProblemJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.RecoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var pd bang.ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if pd.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected payload status: got %d, want %d", pd.Status, http.StatusInternalServerError)
	}
	if pd.Instance != "/panic" {
		t.Fatalf("unexpected instance: got %q", pd.Instance)
	}
	if !strings.Contains(pd.Type, "panic.recovered") {
		t.Fatalf("unexpected type: got %q", pd.Type)
	}
}

func TestRecoveryMiddlewareSkipsWhenAlreadyWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.RecoveryMiddleware())
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
		panic("after write")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusAccepted)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRecoveryMiddlewareWorksWithErrorMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(banggin.ErrorMiddleware(), banggin.RecoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) {
		panic(errors.New("boom"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var pd bang.ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &pd); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if pd.Instance != "/panic" {
		t.Fatalf("unexpected instance: got %q", pd.Instance)
	}
}
