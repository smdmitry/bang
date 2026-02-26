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

func TestConstructorNotFound(t *testing.T) {
	e := bang.NotFound()
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetCode() != "" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetFile() == "" || e.GetLine() == 0 {
		t.Error("expected auto-captured file:line")
	}
}

func TestConstructorWrap(t *testing.T) {
	cause := fmt.Errorf("db down")
	e := bang.Database().Wrap(cause)
	if e.GetCause() != cause {
		t.Error("should wrap cause")
	}
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

func TestAllConstructors(t *testing.T) {
	constructors := []struct {
		name string
		fn   func() *bang.Error
		cls  bang.Class
	}{
		{"InternalServer", bang.InternalServer, bang.ClassInternalServer},
		{"NotFound", bang.NotFound, bang.ClassNotFound},
		{"BadRequest", bang.BadRequest, bang.ClassBadRequest},
		{"Unauthorized", bang.Unauthorized, bang.ClassUnauthorized},
		{"Forbidden", bang.Forbidden, bang.ClassForbidden},
		{"Conflict", bang.Conflict, bang.ClassConflict},
		{"TooManyRequests", bang.TooManyRequests, bang.ClassTooManyRequests},
		{"Database", bang.Database, bang.ClassDatabase},
		{"DuplicateKey", bang.DuplicateKey, bang.ClassDuplicateKey},
		{"ForeignKeyViolation", bang.ForeignKeyViolation, bang.ClassForeignKeyViolation},
		{"ExternalServiceError", bang.ExternalServiceError, bang.ClassExternalService},
		{"NetworkError", bang.NetworkError, bang.ClassNetwork},
		{"TimeoutError", bang.TimeoutError, bang.ClassTimeout},
		{"SerializationError", bang.SerializationError, bang.ClassSerialization},
		{"Unexpected", bang.Unexpected, bang.ClassUnexpected},
		{"NotImplemented", bang.NotImplemented, bang.ClassNotImplemented},
		{"Locked", bang.Locked, bang.ClassLocked},
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

func TestFromInheritsUnsetFieldsFromCause(t *testing.T) {
	cause := bang.Prefix("orders.repo").NotFound().
		Code("get_by_id").
		Title("Order Not Found").
		Msg("Order {{.id}} not found").
		Data(TestData{UserID: "u-1", Query: "SELECT 1"}).
		Safe("id", "u-1").
		Debug("trace_id", "abc-123").
		Field("id", "required", "missing")
	cause = cause.HTTPStatus(499).WithStack()

	e := bang.From(cause)

	if e.GetCause() != cause {
		t.Fatal("cause should be set")
	}
	if e.GetClass() != cause.GetClass() {
		t.Errorf("class = %v, want %v", e.GetClass(), cause.GetClass())
	}
	if e.GetCodePrefix() != cause.GetCodePrefix() {
		t.Errorf("codePrefix = %q, want %q", e.GetCodePrefix(), cause.GetCodePrefix())
	}
	if e.GetCode() != cause.GetCode() {
		t.Errorf("code = %q, want %q", e.GetCode(), cause.GetCode())
	}
	if e.GetTitle() != cause.GetTitle() {
		t.Errorf("title = %q, want %q", e.GetTitle(), cause.GetTitle())
	}
	if e.GetMessage() != cause.GetMessage() {
		t.Errorf("message = %q, want %q", e.GetMessage(), cause.GetMessage())
	}
	if e.GetMsgTpl() != cause.GetMsgTpl() {
		t.Errorf("msgTpl = %q, want %q", e.GetMsgTpl(), cause.GetMsgTpl())
	}
	if e.GetHTTPStatus() != cause.GetHTTPStatus() {
		t.Errorf("httpStatus = %d, want %d", e.GetHTTPStatus(), cause.GetHTTPStatus())
	}
	if e.GetData() == nil {
		t.Error("data should be inherited from cause")
	}
	if e.GetSafe()["id"] != "u-1" {
		t.Errorf("safe[id] = %v", e.GetSafe()["id"])
	}
	if e.GetDebug()["trace_id"] != "abc-123" {
		t.Errorf("debug[trace_id] = %v", e.GetDebug()["trace_id"])
	}
	if len(e.GetValidationFields()) != 1 {
		t.Errorf("validationFields len = %d", len(e.GetValidationFields()))
	}
	if len(e.GetStack()) == 0 {
		t.Error("stack should be inherited from cause")
	}
	if e.GetFile() == cause.GetFile() && e.GetLine() == cause.GetLine() {
		t.Errorf("file:line = %s:%d, want %s:%d", e.GetFile(), e.GetLine(), cause.GetFile(), cause.GetLine())
	}
}

func TestPrefixFactoryFromStoresOnlyCause(t *testing.T) {
	cause := bang.Prefix("source.repo").NotFound().
		Code("item.get").
		Title("Item Not Found").
		Msg("Item {{.id}} not found").
		Safe("id", "42")

	factory := bang.Prefix("factory.repo").
		Class(bang.ClassConflict).
		Title("Factory Title").
		Msg("Factory message").
		HTTPStatus(511)

	e := factory.From(cause)

	if e.GetCause() != cause {
		t.Fatal("cause should be set")
	}
	if e.GetCodePrefix() != cause.GetCodePrefix() {
		t.Errorf("codePrefix = %q, want %q", e.GetCodePrefix(), cause.GetCodePrefix())
	}
	if e.GetClass() != cause.GetClass() {
		t.Errorf("class = %v, want %v", e.GetClass(), cause.GetClass())
	}
	if e.GetCode() != cause.GetCode() {
		t.Errorf("code = %q, want %q", e.GetCode(), cause.GetCode())
	}
	if e.GetTitle() != cause.GetTitle() {
		t.Errorf("title = %q, want %q", e.GetTitle(), cause.GetTitle())
	}
	if e.GetMessage() != cause.GetMessage() {
		t.Errorf("message = %q, want %q", e.GetMessage(), cause.GetMessage())
	}
	if e.GetHTTPStatus() != cause.GetHTTPStatus() {
		t.Errorf("httpStatus = %d, want %d", e.GetHTTPStatus(), cause.GetHTTPStatus())
	}
}

func TestFromFallbackMaxDepth(t *testing.T) {
	base := bang.NotFound().Code("deep.code").Msg("deep message")

	var withinDepth error = base
	for i := 0; i < 5; i++ {
		withinDepth = bang.From(withinDepth)
	}
	if got := withinDepth.(*bang.Error).GetCode(); got != "deep.code" {
		t.Errorf("code within depth = %q, want deep.code", got)
	}

	var overDepth error = base
	for i := 0; i < 6; i++ {
		overDepth = bang.From(overDepth)
	}
	e := overDepth.(*bang.Error)
	if e.GetCode() != "" {
		t.Errorf("code over depth = %q, want empty", e.GetCode())
	}
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class over depth = %q, want %q", e.GetClass(), bang.ClassNotFound)
	}
	if e.GetMessage() != "Запрашиваемый ресурс не найден." {
		t.Errorf("message over depth = %q, want unexpected default", e.GetMessage())
	}
}

func TestSelfAccessorsDoNotFallbackToCause(t *testing.T) {
	cause := bang.Prefix("orders.repo").NotFound().
		Code("get_by_id").
		Title("Order Not Found").
		Msg("Order {{.id}} not found").
		Data(TestData{UserID: "u-1", Query: "SELECT 1"}).
		Safe("id", "u-1").
		Debug("trace_id", "abc-123").
		Field("id", "required", "missing")
	cause = cause.HTTPStatus(499).WithStack()

	e := bang.From(cause)

	if e.SelfClass() != cause.GetClass() {
		t.Errorf("SelfClass = %q, want %q", e.SelfClass(), cause.GetClass())
	}
	if e.SelfCodePrefix() != "" {
		t.Errorf("SelfCodePrefix = %q, want empty", e.SelfCodePrefix())
	}
	if e.SelfCode() == cause.GetCode() {
		t.Errorf("SelfCode should be local, got inherited %q", e.SelfCode())
	}
	if e.SelfTitle() == cause.GetTitle() {
		t.Errorf("SelfTitle should be local, got inherited %q", e.SelfTitle())
	}
	if e.SelfMsgTpl() != "" {
		t.Errorf("SelfMsgTpl = %q, want empty", e.SelfMsgTpl())
	}
	if e.SelfMessage() == cause.GetMessage() {
		t.Errorf("SelfMessage should be local, got inherited %q", e.SelfMessage())
	}
	if e.SelfData() != nil {
		t.Errorf("SelfData = %v, want nil", e.SelfData())
	}
	if e.SelfSafe() != nil {
		t.Errorf("SelfSafe = %v, want nil", e.SelfSafe())
	}
	if e.SelfDebug() != nil {
		t.Errorf("SelfDebug = %v, want nil", e.SelfDebug())
	}
	if len(e.SelfValidationFields()) != 0 {
		t.Errorf("SelfValidationFields len = %d, want 0", len(e.SelfValidationFields()))
	}
	if len(e.SelfStack()) != 0 {
		t.Errorf("SelfStack len = %d, want 0", len(e.SelfStack()))
	}
	if e.SelfHTTPStatus() != 0 {
		t.Errorf("SelfHTTPStatus = %d, want 0", e.SelfHTTPStatus())
	}
}

// ─── Builder methods ────────────────────────────────────────────────────────

func TestBuilderChaining(t *testing.T) {
	cause := fmt.Errorf("original")
	e := bang.NotFound().Wrap(cause).
		Code("users.get").
		Title("Custom Title").
		Msg("Custom message").
		Safe("visible_key", "visible_value").
		Debug("debug_key", "debug_value")

	if e.GetCode() != "users.get" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetTitle() != "Custom Title" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "Custom message" {
		t.Errorf("message = %q", e.GetMessage())
	}
	if e.GetCause() != cause {
		t.Error("should wrap cause")
	}
	if e.GetSafe()["visible_key"] != "visible_value" {
		t.Errorf("safe = %v", e.GetSafe())
	}
	if e.GetDebug()["debug_key"] != "debug_value" {
		t.Errorf("debug = %v", e.GetDebug())
	}
}

func TestWrapDoesNotInheritBangErrorFields(t *testing.T) {
	parent := bang.NotFound().
		Code("users.get_by_id").
		Title("User Not Found").
		Msg("User {{.user_id}} not found").
		Safe("user_id", "u-1")

	e := bang.InternalServer().Wrap(parent)

	if e.GetClass() != bang.ClassInternalServer {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetCode() != "users.get_by_id" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetTitle() != "User Not Found" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "User u-1 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
	if e.GetCause() != parent {
		t.Fatal("cause should be parent")
	}
	if !errors.Is(e, parent) {
		t.Fatal("errors.Is should match parent")
	}
}

func TestWrapInheritBangErrorCanOverride(t *testing.T) {
	parent := bang.NotFound().
		Code("users.get_by_id").
		Title("User Not Found").
		Msg("User not found")

	e := bang.InternalServer().
		Wrap(parent).
		Code("users.lookup_failed").
		Title("Lookup Failed").
		Msg("Lookup failed")

	if e.GetCode() != "users.lookup_failed" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetTitle() != "Lookup Failed" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "Lookup failed" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestSafeMap(t *testing.T) {
	e := bang.NotFound().SafeMap(bang.Map{"k1": "v1", "k2": "v2"})
	safe := e.GetSafe()
	if safe["k1"] != "v1" || safe["k2"] != "v2" {
		t.Errorf("safe = %v", safe)
	}
}

func TestDebugMap(t *testing.T) {
	e := bang.NotFound().DebugMap(bang.Map{"dk": "dv"})
	dbg := e.GetDebug()
	if dbg["dk"] != "dv" {
		t.Errorf("debug = %v", dbg)
	}
}

func TestMsgf(t *testing.T) {
	e := bang.NotFound().Msgf("user %s not found", "abc-123")
	if e.GetMessage() != "user abc-123 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── Template resolution ────────────────────────────────────────────────────

func TestMsgTemplateWithSafe(t *testing.T) {
	e := bang.NotFound().
		Safe("name", "John").
		Msg("Hello {{.name}}")
	if e.GetMessage() != "Hello John" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestMsgTemplateWithDebug(t *testing.T) {
	e := bang.NotFound().
		Debug("sku", "X1", "warehouse", "msk").
		Msg("товар {{.sku}} не найден на складе {{.warehouse}}")
	if e.GetMessage() != "товар X1 не найден на складе msk" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

type TplData struct {
	Name string `json:"name" expose:"true"`
	ID   string `json:"id"`
}

func TestMsgTemplateWithData(t *testing.T) {
	e := bang.NotFound().
		Data(TplData{Name: "John", ID: "u1"}).
		Msg("user {{.name}} (id={{.id}}) not found")
	if e.GetMessage() != "user John (id=u1) not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── Data / SafeData / AllData ──────────────────────────────────────────────

type TestData struct {
	UserID string `json:"userId" expose:"true"`
	Query  string `json:"query"`
}

func TestSafeDataMerge(t *testing.T) {
	e := bang.NotFound().
		Data(TestData{UserID: "u1", Query: "SELECT *"}).
		Safe("extra", "info")

	safe := e.SafeData()
	if safe["userId"] != "u1" {
		t.Errorf("userId = %v", safe["userId"])
	}
	if _, has := safe["query"]; has {
		t.Error("query should not be in safe data")
	}
	if safe["extra"] != "info" {
		t.Errorf("extra = %v", safe["extra"])
	}
}

func TestAllDataMerge(t *testing.T) {
	e := bang.NotFound().
		Data(TestData{UserID: "u1", Query: "SELECT *"}).
		Safe("visible", "yes").
		Debug("hidden", "no")

	all := e.AllData()
	if all["userId"] != "u1" || all["query"] != "SELECT *" {
		t.Errorf("missing typed data: %v", all)
	}
	if all["visible"] != "yes" || all["hidden"] != "no" {
		t.Errorf("missing kv data: %v", all)
	}
}

func TestGetData(t *testing.T) {
	e := bang.InternalServer().Data(TestData{UserID: "u1", Query: "SELECT *"})
	got, ok := bang.GetData[TestData](e)
	if !ok || got.UserID != "u1" {
		t.Errorf("GetData = %+v, %v", got, ok)
	}
}

// ─── error interface ────────────────────────────────────────────────────────

func TestErrorInterface(t *testing.T) {
	var err error = bang.NotFound().Code("test")
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestUnwrap(t *testing.T) {
	cause := fmt.Errorf("db: connection refused")
	e := bang.Database().Wrap(cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

func TestErrorsIs(t *testing.T) {
	e := bang.NotFound().Code("test.code")
	if !errors.Is(e, bang.NotFound()) {
		t.Error("errors.Is(e, bang.NotFound()) should match by class")
	}
}

// ─── Registration ───────────────────────────────────────────────────────────

func TestMustRegisterMessages(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterMessages(map[string]string{
		"test.reg.msg": "Hello {{.name}}",
	})

	e := bang.New("test.reg.msg").Safe("name", "World")
	if e.GetMessage() != "Hello World" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestMustRegisterCodes(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"test.reg.code": {
			Class: bang.ClassNotFound,
			Title: "Custom Title",
			Msg:   "Item {{.id}} not found",
		},
	})

	e := bang.New("test.reg.code").Safe("id", "42")
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetTitle() != "Custom Title" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "Item 42 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestMustRegisterCodesDuplicate(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"test.dup": {Class: bang.ClassNotFound},
	})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate")
		}
	}()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"test.dup": {Class: bang.ClassConflict},
	})
}

func TestFactory(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"tasks.not_found": {
			Class: bang.ClassNotFound,
			Msg:   "Task not found",
		},
	})

	TaskNotFound := bang.Factory("tasks.not_found")
	e := TaskNotFound()
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetCode() != "tasks.not_found" {
		t.Errorf("code = %q", e.GetCode())
	}
}

func TestFactoryIs(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"test.factory.is": {Class: bang.ClassNotFound},
	})

	f := bang.Factory("test.factory.is")
	e := f()
	if !f.Is(e) {
		t.Error("Is should match")
	}
	if f.Is(bang.Conflict()) {
		t.Error("Is should not match different error")
	}
}

func TestRegisterBuilder(t *testing.T) {
	bang.ResetRegistry()
	TaskNotFound := bang.Register("test.register.builder").
		Class(bang.ClassNotFound).
		Message("Task {{.key}} not found").
		Title("Task Error").
		New()

	e := TaskNotFound().Safe("key", "abc")
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetMessage() != "Task abc not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
	if e.GetTitle() != "Task Error" {
		t.Errorf("title = %q", e.GetTitle())
	}
}

func TestTypedFactory(t *testing.T) {
	bang.ResetRegistry()

	type TaskData struct {
		TaskID string `json:"task_id" expose:"true"`
		Owner  string `json:"owner,omitempty"`
	}

	TaskNotFound := bang.Typed[TaskData](
		bang.Register("test.typed.factory").
			Class(bang.ClassNotFound).
			Message("Task {{.task_id}} not found"),
	)

	e := TaskNotFound(TaskData{TaskID: "t-1", Owner: "admin"})
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetMessage() != "Task t-1 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}

	data, ok := bang.GetData[TaskData](e)
	if !ok || data.TaskID != "t-1" {
		t.Errorf("GetData = %+v, %v", data, ok)
	}
}

func TestRegisterTypedBuilder(t *testing.T) {
	bang.ResetRegistry()

	type TaskData struct {
		TaskID string `json:"task_id" expose:"true"`
		Owner  string `json:"owner,omitempty"`
	}

	TaskNotFound := bang.RegisterTyped[TaskData]("test.typed.builder").
		From(bang.NotFound()).
		Message("Task {{.task_id}} not found").
		New()

	e := TaskNotFound(TaskData{TaskID: "t-2", Owner: "admin"})
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetMessage() != "Task t-2 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}

	data, ok := TaskNotFound.GetData(e)
	if !ok || data.TaskID != "t-2" {
		t.Errorf("GetData = %+v, %v", data, ok)
	}
}

func TestTypedFactoryIs(t *testing.T) {
	bang.ResetRegistry()
	type D struct{ X string }
	f := bang.Typed[D](bang.Register("test.typed.is").Class(bang.ClassConflict))

	e := f(D{X: "test"})
	if !f.Is(e) {
		t.Error("Is should match")
	}
}

// ─── Code method applies registry ───────────────────────────────────────────

func TestCodeMethodAppliesRegistry(t *testing.T) {
	bang.ResetRegistry()
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"test.code.apply": {
			Class: bang.ClassForbidden,
			Title: "Applied Title",
			Msg:   "Applied message {{.x}}",
		},
	})

	e := bang.NotFound().Code("test.code.apply").Safe("x", "val")
	if e.GetClass() != bang.ClassForbidden {
		t.Errorf("class = %v, want forbidden", e.GetClass())
	}
	if e.GetTitle() != "Applied Title" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "Applied message val" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── Prefix factory ─────────────────────────────────────────────────────────

func TestPrefixFactoryCode(t *testing.T) {
	repoerr := bang.Prefix("orders.repo")
	e := repoerr.Conflict().Code("archive_failed")
	if e.GetCode() != "orders.repo.archive_failed" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetClass() != bang.ClassConflict {
		t.Errorf("class = %v", e.GetClass())
	}
}

func TestPrefixFactoryDefaults(t *testing.T) {
	repoerr := bang.Prefix("orders.repo").
		Class(bang.ClassNotFound).
		Title("Order Not Found").
		Msg("Order {{.id}} not found")

	e := repoerr.New().Safe("id", "42")
	if e.GetCode() != "orders.repo" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
	if e.GetTitle() != "Order Not Found" {
		t.Errorf("title = %q", e.GetTitle())
	}
	if e.GetMessage() != "Order 42 not found" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestPrefixFactoryWrapDBError(t *testing.T) {
	repoerr := bang.Prefix("orders.repo")
	e := repoerr.WrapDBError(fmt.Errorf("no rows in result set"), "get_by_id")
	if e.GetCode() != "orders.repo.get_by_id" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetClass() != bang.ClassNotFound {
		t.Errorf("class = %v", e.GetClass())
	}
}

func TestPrefixFactoryMainUsage(t *testing.T) {
	repoerr := bang.Prefix("project.archive")
	projectID := "prj-1"

	e := repoerr.Conflict().
		Code("archive_failed").
		Msg("Проект {{.project_id}} не ожидает архивации.").
		Safe("project_id", projectID)

	if e.GetCode() != "project.archive.archive_failed" {
		t.Errorf("code = %q", e.GetCode())
	}
	if e.GetMessage() != "Проект prj-1 не ожидает архивации." {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── Validation ─────────────────────────────────────────────────────────────

func TestValidation(t *testing.T) {
	e := bang.Validation().Code("users.register").
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
	e1 := bang.Validation().Code("v1").Field("a", "r1", "reason a")
	e2 := bang.Validation().Code("v2").Field("b", "r2", "reason b")
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
	e := bang.NotFound().Code("test")
	if !bang.HasClass(e, bang.ClassNotFound) {
		t.Error("should match")
	}
	if bang.HasClass(e, bang.ClassConflict) {
		t.Error("should not match")
	}
}

func TestHasCode(t *testing.T) {
	e := bang.NotFound().Code("my.code")
	if !bang.HasCode(e, "my.code") {
		t.Error("should match")
	}
}

func TestIsValidation(t *testing.T) {
	e := bang.Validation()
	if !bang.IsValidation(e) {
		t.Error("should be validation")
	}
	if bang.IsValidation(bang.NotFound()) {
		t.Error("should not be validation")
	}
}

// ─── Stack ──────────────────────────────────────────────────────────────────

func TestWithStack(t *testing.T) {
	e := bang.InternalServer().WithStack()
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
	e := bang.NotFound().Code("users.get").Msg("not found")
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{Instance: "/api/users/1"})
	if pd.Status != http.StatusNotFound {
		t.Errorf("status = %d", pd.Status)
	}
	if pd.DebugInfo != nil {
		t.Error("no debug without ShowDebug")
	}
}

func TestToHTTPValidation(t *testing.T) {
	e := bang.Validation().Field("email", "fmt", "bad")
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if len(pd.Errors) != 1 {
		t.Errorf("errors=%d", len(pd.Errors))
	}
	// Rule should not be in the HTTP response.
	if pd.Errors[0].Key != "email" {
		t.Errorf("key = %q", pd.Errors[0].Key)
	}
}

func TestToHTTPDebug(t *testing.T) {
	e := bang.InternalServer().
		Data(TestData{UserID: "u1", Query: "q"}).
		Safe("public", "visible").
		Debug("extra", "info").
		WithStack()
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{ShowDebug: true})
	if pd.DebugInfo == nil {
		t.Fatal("debug should be present")
	}
	if pd.DebugInfo.AllData == nil || len(pd.DebugInfo.Stack) == 0 {
		t.Error("debug data/stack missing")
	}
	if pd.DebugInfo.AllData["query"] != "q" {
		t.Errorf("query = %v", pd.DebugInfo.AllData["query"])
	}
	if pd.DebugInfo.AllData["extra"] != "info" {
		t.Errorf("extra = %v", pd.DebugInfo.AllData["extra"])
	}
	if _, has := pd.DebugInfo.AllData["userId"]; has {
		t.Error("safe typed field userId should not be in debug data")
	}
	if _, has := pd.DebugInfo.AllData["public"]; has {
		t.Error("safe map field public should not be in debug data")
	}
}

func TestToHTTPCauseChain(t *testing.T) {
	c6 := bang.InternalServer().Code("c6").Msg("cause 6")
	c5 := bang.InternalServer().Wrap(c6).Code("c5").Msg("cause 5")
	c4 := bang.InternalServer().Wrap(c5).Code("c4").Msg("cause 4")
	c3 := bang.InternalServer().Wrap(c4).Code("c3").Msg("cause 3")
	c2 := bang.InternalServer().Wrap(c3).Code("c2").Msg("cause 2")
	c1 := bang.InternalServer().Wrap(c2).Code("c1").Msg("cause 1")
	top := bang.InternalServer().Wrap(c1).Code("top").Msg("top")

	pd := bang.ToHTTP(top, bang.HTTPResponseOptions{})
	if len(pd.Cause) != 5 {
		t.Fatalf("cause len = %d, want 5", len(pd.Cause))
	}
	if pd.Cause[0].Type != "urn:internal_server:c1" || pd.Cause[4].Type != "urn:internal_server:c5" {
		t.Errorf("unexpected cause types: first=%q last=%q", pd.Cause[0].Type, pd.Cause[4].Type)
	}
	if pd.Cause[0].Detail != "cause 1" {
		t.Errorf("detail = %q", pd.Cause[0].Detail)
	}

	raw, err := json.Marshal(pd.Cause[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["status"]; ok {
		t.Error("cause should not contain status")
	}
	if _, ok := m["instance"]; ok {
		t.Error("cause should not contain instance")
	}

	pdDebug := bang.ToHTTP(top, bang.HTTPResponseOptions{ShowDebug: true})
	if pdDebug.DebugInfo == nil {
		t.Fatal("debug should be present")
	}
	if pdDebug.DebugInfo.Cause != "" {
		t.Errorf("debug cause should be empty for bang-only chain, got %q", pdDebug.DebugInfo.Cause)
	}
}

func TestToHTTPCauseChainSkipsNonBang(t *testing.T) {
	root := fmt.Errorf("io fail")
	e := bang.InternalServer().Wrap(root)
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if len(pd.Cause) != 0 {
		t.Fatalf("cause len = %d, want 0", len(pd.Cause))
	}

	pdDebug := bang.ToHTTP(e, bang.HTTPResponseOptions{ShowDebug: true})
	if pdDebug.DebugInfo == nil {
		t.Fatal("debug should be present")
	}
	if pdDebug.DebugInfo.Cause != "io fail" {
		t.Errorf("debug cause = %q, want %q", pdDebug.DebugInfo.Cause, "io fail")
	}
}

func TestToHTTPCauseChainMixedBangAndNonBang(t *testing.T) {
	root := fmt.Errorf("io fail")
	c1 := bang.InternalServer().Wrap(root).Code("c1").Msg("cause 1")
	top := bang.InternalServer().Wrap(c1).Code("top").Msg("top")

	pd := bang.ToHTTP(top, bang.HTTPResponseOptions{})
	if len(pd.Cause) != 1 {
		t.Fatalf("cause len = %d, want 1", len(pd.Cause))
	}
	if pd.Cause[0].Type != "urn:internal_server:c1" {
		t.Errorf("cause type = %q", pd.Cause[0].Type)
	}

	pdDebug := bang.ToHTTP(top, bang.HTTPResponseOptions{ShowDebug: true})
	if pdDebug.DebugInfo == nil {
		t.Fatal("debug should be present")
	}
	if pdDebug.Cause[0].DebugInfo.Cause != "io fail" {
		t.Errorf("debug cause = %q, want %q", pdDebug.DebugInfo.Cause, "io fail")
	}
}

func TestToHTTPSafeOnly(t *testing.T) {
	e := bang.InternalServer().Data(TestData{UserID: "u1", Query: "SELECT *"})
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if _, has := pd.Details["query"]; has {
		t.Error("query should not be in safe details")
	}
}

func TestWriteHTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	bang.WriteHTTP(rec, bang.NotFound().Code("test"), bang.HTTPResponseOptions{Instance: "/test"})
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
	e := bang.InternalServer().Code("test.log").Debug("k", "v")
	rec := bang.ToLogRecord(e)
	if rec.Class != "internal_server" || rec.Code != "test.log" {
		t.Errorf("class=%q code=%q", rec.Class, rec.Code)
	}
}

func TestToJSON(t *testing.T) {
	j := bang.ToJSON(bang.NotFound().Code("test"))
	var m map[string]any
	if err := json.Unmarshal([]byte(j), &m); err != nil {
		t.Fatal(err)
	}
}

func TestSlogAttrs(t *testing.T) {
	attrs := bang.SlogAttrs(bang.InternalServer().Code("test"))
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
	orig := bang.NotFound().Code("test")
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
		{"connection refused", bang.ClassDatabase},
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
	e := bang.FromClass(MyClass).Code("test.custom")
	if e.GetTitle() != "My Custom Error" {
		t.Errorf("title = %q", e.GetTitle())
	}
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if pd.Status != 418 {
		t.Errorf("status = %d", pd.Status)
	}
}

// ─── Template missing key mode ──────────────────────────────────────────────

func TestTemplateMissingKeyKeep(t *testing.T) {
	bang.SetTemplateMissingKeyMode(bang.MissingKeyKeep)
	defer bang.SetTemplateMissingKeyMode(bang.MissingKeyKeep)

	e := bang.NotFound().Msg("Hello {{.missing}}")
	if e.GetMessage() != "Hello {{.missing}}" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

func TestTemplateMissingKeyRemove(t *testing.T) {
	bang.SetTemplateMissingKeyMode(bang.MissingKeyRemove)
	defer bang.SetTemplateMissingKeyMode(bang.MissingKeyKeep)

	e := bang.NotFound().Msg("Hello {{.missing}} world")
	if e.GetMessage() != "Hello  world" {
		t.Errorf("message = %q", e.GetMessage())
	}
}

// ─── AsBang ─────────────────────────────────────────────────────────────────

func TestAsBang(t *testing.T) {
	e := bang.NotFound().Code("test")
	got, ok := bang.AsBang(e)
	if !ok || got != e {
		t.Error("AsBang should return the error")
	}

	_, ok = bang.AsBang(fmt.Errorf("not bang"))
	if ok {
		t.Error("AsBang should return false for non-bang")
	}
}

// ─── HTTPStatus override ────────────────────────────────────────────────────

func TestHTTPStatusOverride(t *testing.T) {
	e := bang.NotFound().HTTPStatus(418)
	pd := bang.ToHTTP(e, bang.HTTPResponseOptions{})
	if pd.Status != 418 {
		t.Errorf("status = %d, want 418", pd.Status)
	}
}
