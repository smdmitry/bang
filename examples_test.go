package bang_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/smdmitry/bang"
)

// ═══════════════════════════════════════════════════════════════════════════
//  1. ИСПОЛЬЗОВАНИЕ НАПРЯМУЮ
// ═══════════════════════════════════════════════════════════════════════════

func Example_directUsage() {
	var err error

	// Самый простой вызов
	_ = bang.NotFound()

	// С кодом
	_ = bang.NotFound().Code("users.get_by_id")

	// Wrap ошибки
	_ = bang.Database().Wrap(err)

	// Полный пример с fluent builder
	_ = bang.NotFound().Wrap(err).
		SafeMap(bang.Map{"key": "value"}).
		Safe("key", "value").
		DebugMap(bang.Map{"key": "value"}).
		Debug("keyd", "valued").
		Code("можно.задать.код").
		Title("Можно переопределить заголовок").
		Msg("Можно переопределить сообщение")

	// Шаблон с подстановками
	_ = bang.NotFound().
		Safe("key", "task-123").
		Msg("Можно переопределить сообщение {{.key}}")
}

// ═══════════════════════════════════════════════════════════════════════════
//  2. ПРОСТАЯ РЕГИСТРАЦИЯ
// ═══════════════════════════════════════════════════════════════════════════

const CodeTaskNotFound = "task.not_found"

func Example_simpleRegistration() {
	bang.ResetRegistry()

	// Можно зарегать только сообщение
	bang.MustRegisterMessages(map[string]string{
		CodeTaskNotFound: "Task '{{.key}}' not found",
	})

	// Использование через New
	e := bang.New(CodeTaskNotFound).Safe("key", "task-1")
	fmt.Println(e.GetMessage())
	// Output: Task 'task-1' not found
}

func Example_fullRegistration() {
	bang.ResetRegistry()

	// Полная регистрация кода
	bang.MustRegisterCodes(map[string]bang.CodeCfg{
		"task.not_found_full": {
			Class: bang.ClassNotFound,
			Title: "Задача не найдена",
			Msg:   "Task '{{.key}}' not found",
		},
	})

	// Factory
	TaskNotFound := bang.Factory("task.not_found_full")
	e := TaskNotFound().Safe("key", "task-1")
	fmt.Println(e.GetClass(), e.GetTitle())
	// Output: not_found Задача не найдена
}

// ═══════════════════════════════════════════════════════════════════════════
//  3. СОЗДАНИЕ ТИПА ЧЕРЕЗ REGISTER BUILDER
// ═══════════════════════════════════════════════════════════════════════════

func Example_registerBuilder() {
	bang.ResetRegistry()

	TaskNotFound := bang.Register("task.not_found_builder").
		From(bang.NotFound()).
		Class(bang.ClassNotFound).
		Message("Task '{{.key}}' not found").
		Title("Задача не найдена").
		New()

	e := TaskNotFound().Safe("key", "abc-123")
	fmt.Println(e.GetMessage())
	// Output: Task 'abc-123' not found
}

// ═══════════════════════════════════════════════════════════════════════════
//  4. ТИПИЗИРОВАННАЯ ОШИБКА
// ═══════════════════════════════════════════════════════════════════════════

type TaskNotFoundData struct {
	TaskID string `json:"task_id" expose:"true"`
	Owner  string `json:"owner,omitempty"`
}

func Example_typedFactory() {
	bang.ResetRegistry()

	TaskNotFound := bang.RegisterTyped[TaskNotFoundData]("task.not_found_typed").
		From(bang.NotFound()).
		Message("Task '{{.task_id}}' not found").
		New()

	// Данные передать обязательно
	e := TaskNotFound(TaskNotFoundData{TaskID: "t-1", Owner: "admin"})
	fmt.Println(e.GetMessage())
	// Output: Task 't-1' not found
}

// ═══════════════════════════════════════════════════════════════════════════
//  5. MODULE
// ═══════════════════════════════════════════════════════════════════════════

var ordersRepo = bang.NewModule("orders.repo")

func Example_module() {
	var err error

	_ = ordersRepo.NotFound().Code("get_by_id")
	_ = ordersRepo.WrapDBError(err, "update")
	_ = ordersRepo.WrapRedisError(err, "cache")
}

// ═══════════════════════════════════════════════════════════════════════════
//  6. VALIDATION
// ═══════════════════════════════════════════════════════════════════════════

func Example_validation() {
	// Простая валидация
	_ = bang.Validation().Code("users.register").
		Field("email", "email_format", "Введите корректный email.").
		Field("password", "min_length", "Пароль — не менее 8 символов.")

	// Комбинирование валидаций
	err1 := bang.Validation().Code("passport").
		Field("passports.0.issueDate", "passports_date_format",
			"Исправьте дату выдачи. В качестве разделителя между днём, месяцем и годом используйте точку.")
	err2 := bang.Validation().Code("passport").
		Field("passports.1.serial", "passports_serial_format",
			"Исправьте серию паспорта. Серия паспорта - это четыре арабские цифры")

	combined := bang.CombineValidation(err1, err2)
	fields := combined.GetValidationFields()
	fmt.Println(len(fields))
	// Output: 2
}

// ═══════════════════════════════════════════════════════════════════════════
//  7. DB / REDIS ADAPTERS
// ═══════════════════════════════════════════════════════════════════════════

func Example_adapters() {
	var err error

	// WrapDBError: auto-class
	_ = bang.WrapDBError(err).Code("users.repo.get").Debug("id", "u1")

	// Через Module
	_ = ordersRepo.WrapDBError(err, "get_by_id").Debug("id", "ord-1")

	// WrapRedisError
	_ = bang.WrapRedisError(err).Code("cache.session.get").Debug("sid", "s-123")
}

// ═══════════════════════════════════════════════════════════════════════════
//  8. CUSTOM CLASS
// ═══════════════════════════════════════════════════════════════════════════

const ClassPaymentDeclined bang.Class = "payment_declined"

func init() {
	bang.RegisterClass(ClassPaymentDeclined, bang.ClassDef{
		DefaultCode: "payment_declined",
		Title:       "Payment Declined",
		Message:     "Платёж отклонён банком.",
		HTTPStatus:  402,
	})
}

func Example_customClass() {
	e := bang.FromClass(ClassPaymentDeclined).Code("billing.charge").
		Debug("card", "***1234", "reason", "insufficient_funds")
	fmt.Println(e.GetTitle())
	// Output: Payment Declined
}

// ═══════════════════════════════════════════════════════════════════════════
//  9. HTTP HANDLER
// ═══════════════════════════════════════════════════════════════════════════

func Example_httpHandler() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		err := doSomething(r.Context())
		if err != nil {
			bang.SlogError(slog.Default(), "request failed", err)

			bang.WriteHTTP(w, err, bang.HTTPResponseOptions{
				Instance:  r.URL.Path,
				ShowDebug: false, // isDev || isAdmin
			})
			return
		}
	}
	_ = handler
}

// ═══════════════════════════════════════════════════════════════════════════
//  10. ПРОВЕРКА ОШИБОК
// ═══════════════════════════════════════════════════════════════════════════

func Example_inspection() {
	e := bang.NotFound().Code("users.not_found")
	var err error = e

	// По классу
	fmt.Println(bang.HasClass(err, bang.ClassNotFound))

	// По коду
	fmt.Println(bang.HasCode(err, "users.not_found"))

	// IsValidation
	fmt.Println(bang.IsValidation(err))

	// AsBang
	if be, ok := bang.AsBang(err); ok {
		_ = be.GetFile()
		_ = be.GetLine()
	}

	// Output:
	// true
	// true
	// false
}

func doSomething(_ context.Context) error {
	return fmt.Errorf("example")
}
