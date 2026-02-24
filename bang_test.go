package bang_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smdmitry/bang"
)

// ─── Constructors ───────────────────────────────────────────────────────────

func TestConstructorNoArgs(t *testing.T) {
	e := bang.NotFound()
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetCode() != "not_found" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetFile() == "" || e.GetLine() == 0 {
		t.Error("expected auto-captured file:line")
	}
}

func TestConstructorCodeOnly(t *testing.T) {
	e := bang.NotFound("users.get_by_id")
	if e.GetCode() != "users.get_by_id" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetCause() != nil {
		t.Error("should have no cause")
	}
}

func TestConstructorErrOnly(t *testing.T) {
	cause := fmt.Errorf("db down")
	e := bang.DatabaseError(cause)
	if e.GetCause() != cause {
		t.Error("should wrap cause")
	}
	if e.GetCode() != "database_error" {
		t.Errorf("code = %q (should be default)", e.GetCode())
	}
}

func TestConstructorErrAndCode(t *testing.T) {
	cause := fmt.Errorf("timeout")
	e := bang.SerializationError(cause, "config.parse")
	if e.GetCause() != cause {
		t.Error("should wrap cause")
	}
	if e.GetCode() != "config.parse" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestAllConstructors(t *testing.T) {
	constructors := []struct {
		name string
		fn   func(...any) *bang.Error
		cls  bang.Class
	}{
		{"InternalServer", bang.InternalServer, bang.ClassInternalServer},
		{"NotFound", bang.NotFound, bang.ClassNotFound},
		{"Unauthorized", bang.Unauthorized, bang.ClassUnauthorized},
		{"Forbidden", bang.Forbidden, bang.ClassForbidden},
		{"Conflict", bang.Conflict, bang.ClassConflict},
		{"TooManyRequests", bang.TooManyRequests, bang.ClassTooManyRequests},
		{"DatabaseError", bang.DatabaseError, bang.ClassDatabaseError},
		{"DuplicateKey", bang.DuplicateKey, bang.ClassDuplicateKey},
		{"ForeignKeyViolation", bang.ForeignKeyViolation, bang.ClassForeignKeyViolation},
		{"ExternalServiceError", bang.ExternalServiceError, bang.ClassExternalServiceError},
		{"NetworkError", bang.NetworkError, bang.ClassNetworkError},
		{"TimeoutError", bang.TimeoutError, bang.ClassTimeoutError},
		{"SerializationError", bang.SerializationError, bang.ClassSerializationError},
		{"Unexpected", bang.Unexpected, bang.ClassUnexpected},
	}
	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fn()
			if e.GetClass() != tt.cls {
				t.Errorf("class = %v, want %v", e.GetClass(), tt.cls)
			}
		})
	}
}

// ─── Builder methods ────────────────────────────────────────────────────────

func TestMsgf(t *testing.T) {
	e := bang.NotFound("users.get").Msgf("user %s not found", "abc-123")
	if e.GetMessage() != "user abc-123 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestDebugKV(t *testing.T) {
	e := bang.NotFound("test").DebugKV("id", "123", "table", "users")
	m, ok := e.GetDebug().(map[string]any)
	if !ok {
		t.Fatal("debug should be map")
	}
	if m["id"] != "123" || m["table"] != "users" {
		t.Errorf("debug = %v", m)
	}
}

// ─── MsgTpl ─────────────────────────────────────────────────────────────────

func TestMsgTplWithDebugKV(t *testing.T) {
	e := bang.NotFound("products.get").
		DebugKV("sku", "X1", "warehouse", "msk").
		MsgTpl("товар {sku} не найден на складе {warehouse}")
	if e.GetMessage() != "товар X1 не найден на складе msk" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

type TplData struct {
	Name string `json:"name" expose:"true"`
	ID   string `json:"id"`
}

func TestMsgTplWithData(t *testing.T) {
	e := bang.NotFound("users.get").
		Data(TplData{Name: "John", ID: "u1"}).
		MsgTpl("user {name} (id={id}) not found")
	if e.GetMessage() != "user John (id=u1) not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestMsgTplAutoResolveOnData(t *testing.T) {
	// MsgTpl set first (via Define), Data set later — should auto-resolve.
	factory := bang.Define(bang.ClassNotFound, "test.tpl",
		bang.WithMsgTpl("hello {name}"),
	)
	e := factory().Data(TplData{Name: "World"})
	if e.GetMessage() != "hello World" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestMsgTplAutoResolveOnDebugKV(t *testing.T) {
	factory := bang.Define(bang.ClassDatabaseError, "test.tpl2",
		bang.WithMsgTpl("failed to {op} in {table}"),
	)
	e := factory().DebugKV("op", "insert", "table", "orders")
	if e.GetMessage() != "failed to insert in orders" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── error interface ────────────────────────────────────────────────────────

func TestErrorInterface(t *testing.T) {
	var err error = bang.NotFound("test")
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestUnwrap(t *testing.T) {
	cause := fmt.Errorf("db: connection refused")
	e := bang.DatabaseError(cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

// ─── Data ───────────────────────────────────────────────────────────────────

type TestData struct {
	UserID string `json:"userId" expose:"true"`
	Query  string `json:"query"`
}

func TestGetData(t *testing.T) {
	e := bang.InternalServer("test").Data(TestData{UserID: "u1", Query: "SELECT *"})
	got, ok := bang.GetData[TestData](e)
	if !ok || got.UserID != "u1" {
		t.Errorf("GetData = %+v, %v", got, ok)
	}
}

// ─── Define / DefineTyped ───────────────────────────────────────────────────

var ErrUserNotFound = bang.Define(bang.ClassNotFound, "users.not_found")

func TestDefineCall(t *testing.T) {
	e := ErrUserNotFound()
	if e.GetCode() != "users.not_found" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestDefineWrap(t *testing.T) {
	cause := fmt.Errorf("underlying")
	e := ErrUserNotFound(cause)
	if e.GetCause() != cause {
		t.Error("should wrap cause")
	}
}

func TestDefineIs(t *testing.T) {
	e := ErrUserNotFound()
	if !ErrUserNotFound.Is(e) {
		t.Error("Is should match")
	}
}

var ErrOrderFailed = bang.DefineTyped[TestData](bang.ClassDatabaseError, "orders.save_failed")

func TestDefineTyped(t *testing.T) {
	e := ErrOrderFailed().Data(TestData{UserID: "u1", Query: "INSERT"})
	got, ok := ErrOrderFailed.GetData(e)
	if !ok || got.UserID != "u1" {
		t.Errorf("GetData = %+v, %v", got, ok)
	}
}

func TestDefineTypedIs(t *testing.T) {
	e := ErrOrderFailed()
	if !ErrOrderFailed.Is(e) {
		t.Error("Is should match")
	}
}

func TestDefineWithOptions(t *testing.T) {
	f := bang.Define(bang.ClassConflict, "test.opts",
		bang.WithTitle("Custom Title"),
		bang.WithMessage("Custom message"),
	)
	e := f()
	if e.GetTitle() != "Custom Title" || e.GetMessage() != "Custom message" {
		t.Errorf("title=%q message=%q", e.GetTitle(), e.GetMessage())
	}
}

// ─── Module ─────────────────────────────────────────────────────────────────

func TestModuleConstructor(t *testing.T) {
	mod := bang.NewModule("orders.repo")
	e := mod.NotFound(nil, "get_by_id")
	if e.GetCode() != "orders.repo.get_by_id" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestModuleConstructorNoCode(t *testing.T) {
	mod := bang.NewModule("orders.repo")
	e := mod.DatabaseError(fmt.Errorf("fail"))
	if e.GetCode() != "orders.repo" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestModuleDefine(t *testing.T) {
	mod := bang.NewModule("users.service")
	f := mod.Define(bang.ClassNotFound, "not_found")
	e := f()
	if e.GetCode() != "users.service.not_found" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestModuleWrapDBError(t *testing.T) {
	mod := bang.NewModule("orders.repo")
	e := mod.WrapDBError(fmt.Errorf("no rows in result set"), "get_by_id")
	if e.GetCode() != "orders.repo.get_by_id" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
}

// ─── Validation ─────────────────────────────────────────────────────────────

func TestValidation(t *testing.T) {
	e := bang.Validation("users.register").
		Field("email", "email_format", "bad email").
		Field("password", "min_length", "too short")
	fields := e.GetValidationFields()
	if len(fields) != 2 {
		t.Fatalf("fields count = %d", len(fields))
	}
	if fields[0].Key != "email" {
		t.Errorf("field[0].Key = %q", fields[0].Key)
	}
}

func TestCombineValidation(t *testing.T) {
	e1 := bang.Validation("v1").Field("a", "r1", "reason a")
	e2 := bang.Validation("v2").Field("b", "r2", "reason b")
	combined := bang.CombineValidation(e1, e2)
	if combined == nil || len(combined.GetValidationFields()) != 2 {
		t.Error("should combine fields")
	}
}

func TestCombineValidationAllNil(t *testing.T) {
	if bang.CombineValidation(nil, nil) != nil {
		t.Error("should be nil")
	}
}

// ─── HasClass / HasCode ─────────────────────────────────────────────────────

func TestHasClass(t *testing.T) {
	e := bang.NotFound("test")
	if !bang.HasClass(e, bang.ClassNotFound) {
		t.Error("should match")
	}
	if bang.HasClass(e, bang.ClassConflict) {
		t.Error("should not match")
	}
}

func TestHasCode(t *testing.T) {
	e := bang.NotFound("my.code")
	if !bang.HasCode(e, "my.code") {
		t.Error("should match")
	}
}

// ─── Stack ──────────────────────────────────────────────────────────────────

func TestWithStack(t *testing.T) {
	e := bang.InternalServer("test").WithStack()
	if len(e.StackFrames()) == 0 {
		t.Error("expected stack frames")
	}
}

// ─── Expose ─────────────────────────────────────────────────────────────────

func TestFilterSafe(t *testing.T) {
	safe := bang.FilterSafe(TestData{UserID: "u1", Query: "SELECT *"})
	if safe["userId"] != "u1" {
		t.Errorf("userId = %v", safe["userId"])
	}
	if _, has := safe["query"]; has {
		t.Error("query should not be in safe data")
	}
}

func TestToMap(t *testing.T) {
	m := bang.ToMap(TestData{UserID: "u1", Query: "q"})
	if m["userId"] != "u1" || m["query"] != "q" {
		t.Errorf("ToMap = %v", m)
	}
}

// ─── HTTP ───────────────────────────────────────────────────────────────────

func TestToHTTP(t *testing.T) {
	e := bang.NotFound("users.get").Msg("not found")
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{Instance: "/api/users/1"})
	if pd.Status != http.StatusNotFound {
		t.Errorf("status = %d", pd.Status)
	}
	if pd.ErrorType != "business" {
		t.Errorf("errorType = %q", pd.ErrorType)
	}
	if pd.DebugInfo != nil {
		t.Error("no debug without ShowDebug")
	}
}

func TestToHTTPValidation(t *testing.T) {
	e := bang.Validation("test").Field("email", "fmt", "bad")
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if pd.ErrorType != "validation" || len(pd.Errors) != 1 {
		t.Errorf("errorType=%q errors=%d", pd.ErrorType, len(pd.Errors))
	}
}

func TestToHTTPDebug(t *testing.T) {
	e := bang.InternalServer("test").
		Data(TestData{UserID: "u1", Query: "q"}).
		Debug(map[string]any{"extra": "info"}).
		WithStack()
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{ShowDebug: true})
	if pd.DebugInfo == nil {
		t.Fatal("debug should be present")
	}
	if pd.DebugInfo.AllData == nil || len(pd.DebugInfo.Stack) == 0 {
		t.Error("debug data/stack missing")
	}
}

func TestToHTTPSafeOnly(t *testing.T) {
	e := bang.InternalServer("test").Data(TestData{UserID: "u1", Query: "SELECT *"})
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if _, has := pd.Details["query"]; has {
		t.Error("query should not be in safe details")
	}
}

func TestWriteHTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	bang.WriteHTTP(rec, bang.NotFound("test"), bang.HTTPResponseOptions{Instance: "/test"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q", ct)
	}
	var pd bang.ProblemDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &pd); err != nil {
		t.Fatal(err)
	}
}

// ─── Log ────────────────────────────────────────────────────────────────────

func TestToLogRecord(t *testing.T) {
	e := bang.InternalServer("test.log").Debug(map[string]any{"k": "v"})
	rec := bang.ToLogRecord(e)
	if rec.Class != "internal_server" || rec.Code != "test.log" {
		t.Errorf("class=%q code=%q", rec.Class, rec.Code)
	}
}

func TestToJSON(t *testing.T) {
	j := bang.ToJSON(bang.NotFound("test"))
	var m map[string]any
	if err := json.Unmarshal([]byte(j), &m); err != nil {
		t.Fatal(err)
	}
}

func TestSlogAttrs(t *testing.T) {
	attrs := bang.SlogAttrs(bang.InternalServer("test"))
	if len(attrs) == 0 {
		t.Error("should have attrs")
	}
}

// ─── WrapUnknown ────────────────────────────────────────────────────────────

func TestWrapUnknown(t *testing.T) {
	raw := fmt.Errorf("random")
	e := bang.WrapUnknown(raw)
	if e.GetClass() != bang.ClassUnexpected || !errors.Is(e, raw) {
		t.Error("should wrap as unexpected")
	}
}

func TestWrapUnknownNil(t *testing.T) {
	if bang.WrapUnknown(nil) != nil {
		t.Error("should return nil")
	}
}

func TestWrapUnknownBang(t *testing.T) {
	orig := bang.NotFound("test")
	if bang.WrapUnknown(orig) != orig {
		t.Error("should return same *Error")
	}
}

// ─── DB adapter ─────────────────────────────────────────────────────────────

func TestWrapDBError(t *testing.T) {
	tests := []struct {
		msg   string
		class bang.Class
	}{
		{"no rows in result set", bang.ClassNotFound},
		{"duplicate key value violates unique constraint", bang.ClassDuplicateKey},
		{"insert violates foreign key constraint", bang.ClassForeignKeyViolation},
		{"connection refused", bang.ClassDatabaseError},
	}
	for _, tt := range tests {
		e := bang.WrapDBError(fmt.Errorf(tt.msg))
		if e.GetClass() != tt.class {
			t.Errorf("msg=%q class=%v want=%v", tt.msg, e.GetClass(), tt.class)
		}
	}
}

func TestWrapDBErrorNil(t *testing.T) {
	if bang.WrapDBError(nil) != nil {
		t.Error("should return nil")
	}
}

// ─── Redis adapter ──────────────────────────────────────────────────────────

func TestWrapRedisError(t *testing.T) {
	e := bang.WrapRedisError(fmt.Errorf("redis: nil"))
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
}

func TestWrapRedisErrorNil(t *testing.T) {
	if bang.WrapRedisError(nil) != nil {
		t.Error("should return nil")
	}
}

// ─── RegisterClass ──────────────────────────────────────────────────────────

func TestRegisterClass(t *testing.T) {
	const MyClass bang.Class = "my_custom"
	bang.RegisterClass(MyClass, bang.ClassDef{
		DefaultCode: "my_custom",
		Title:       "My Custom Error",
		Message:     "Something custom.",
		HTTPStatus:  418,
	})
	e := bang.New(MyClass, "test.custom")
	if e.GetTitle() != "My Custom Error" {
		t.Errorf("title = %q", e.GetTitle())
	}
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if pd.Status != 418 {
		t.Errorf("status = %d", pd.Status)
	}
}
