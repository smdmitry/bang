package bang_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/smdmitry/bang"
)

// ═══════════════════════════════════════════════════════════════════════════
//  1. ПРЯМОЕ ИСПОЛЬЗОВАНИЕ — самый короткий путь
// ═══════════════════════════════════════════════════════════════════════════

func Example_direct_minimal() {
	// Только класс — 1 вызов
	_ = bang.NotFound()

	// Класс + код — 1 вызов
	_ = bang.NotFound("users.get_by_id")

	// Wrap ошибки — 1 вызов
	var err error
	_ = bang.DatabaseError(err)

	// Wrap + код — 1 вызов
	_ = bang.SerializationError(err, "config.parse")
}

func Example_direct_with_details() {
	var err error
	id := "u-123"

	// Wrap + код + форматированное сообщение — 2 вызова
	_ = bang.NotFound(err, "users.get_by_id").Msgf("user %s not found", id)

	// Wrap + код + debug — 2 вызова
	_ = bang.DatabaseError(err, "users.create").DebugKV("email", "j@x.com")

	// Код + сообщение + debug — 3 вызова
	_ = bang.Conflict("orders.place").
		Msg("Товар закончился на складе").
		DebugKV("productID", "p1", "warehouse", "msk")

	// Критичная ошибка со стеком — 3 вызова
	_ = bang.InternalServer(err, "payments.charge").
		WithStack().
		DebugKV("orderID", "ord-1", "provider", "stripe")
}

// ═══════════════════════════════════════════════════════════════════════════
//  2. MODULE — общий префикс, короткие коды
// ═══════════════════════════════════════════════════════════════════════════

// --- файл: orders/repo/repo.go ---

var ordersRepo = bang.NewModule("orders.repo")

func Example_module() {
	var err error

	_ = ordersRepo.NotFound(err, "get_by_id")   // → code: orders.repo.get_by_id
	_ = ordersRepo.DatabaseError(err, "create") // → code: orders.repo.create
	_ = ordersRepo.DatabaseError(err)           // → code: orders.repo
	_ = ordersRepo.WrapDBError(err, "update")   // → auto-class + code: orders.repo.update
	_ = ordersRepo.WrapRedisError(err, "cache") // → auto-class + code: orders.repo.cache
}

// ═══════════════════════════════════════════════════════════════════════════
//  3. DEFINE — переиспользуемые фабрики
// ═══════════════════════════════════════════════════════════════════════════

// --- файл: users/errors.go ---

var usersMod = bang.NewModule("users")

var (
	ErrUserNotFound = usersMod.Define(bang.ClassNotFound, "not_found")
	ErrUserExists   = usersMod.Define(bang.ClassConflict, "already_exists",
		bang.WithMessage("Пользователь с таким email уже зарегистрирован."),
	)
	ErrUserDeactivated = usersMod.Define(bang.ClassForbidden, "deactivated",
		bang.WithMessage("Аккаунт деактивирован. Обратитесь в поддержку."),
	)
)

func Example_define() {
	var err error
	id := "u-123"

	_ = ErrUserNotFound()                               // → code: users.not_found
	_ = ErrUserNotFound(err)                            // wrap
	_ = ErrUserNotFound().Msgf("user %s not found", id) // + сообщение
	_ = ErrUserNotFound(err).DebugKV("id", id)          // wrap + debug
	_ = ErrUserExists()                                 // сообщение из Define
	_ = ErrUserDeactivated()                            // сообщение из Define
}

// ═══════════════════════════════════════════════════════════════════════════
//  4. DEFINE TYPED — фабрики с типизированными данными
// ═══════════════════════════════════════════════════════════════════════════

type OrderData struct {
	OrderID string `json:"orderId" expose:"true"`
	Status  string `json:"status"  expose:"true"`
	Query   string `json:"query"` // unsafe
}

var ordersMod = bang.NewModule("orders")

var (
	ErrOrderNotFound = bang.ModuleDefineTyped[OrderData](ordersMod, bang.ClassNotFound, "not_found")
	ErrOrderConflict = bang.ModuleDefineTyped[OrderData](ordersMod, bang.ClassConflict, "status_conflict",
		bang.WithMessage("Невозможно выполнить операцию в текущем статусе заказа."),
	)
)

func Example_defineTyped() {
	var err error

	// Создание + typed data — 2 вызова
	_ = ErrOrderNotFound(err).Data(OrderData{OrderID: "ord-1"})

	// Typed data + debug не нужен, всё в Data
	_ = ErrOrderConflict().Data(OrderData{OrderID: "ord-1", Status: "shipped"})

	// Извлечение
	var someErr error = ErrOrderConflict().Data(OrderData{OrderID: "ord-1", Status: "shipped"})
	if data, ok := ErrOrderConflict.GetData(someErr); ok {
		_ = data.OrderID // "ord-1"
		_ = data.Status  // "shipped"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  5. MSG TEMPLATE — сообщение с подстановками из данных
// ═══════════════════════════════════════════════════════════════════════════

// Шаблон в Define — резолвится автоматически при .Data() / .DebugKV()
var ErrProductMissing = bang.Define(bang.ClassNotFound, "products.missing",
	bang.WithMsgTpl("товар {sku} не найден на складе {warehouse}"),
)

var ErrTableOp = bang.Define(bang.ClassDatabaseError, "db.table_op",
	bang.WithMsgTpl("failed to {op} in {table}"),
)

func Example_msgTpl() {
	var err error

	// Шаблон из Define + DebugKV → авторезолв
	_ = ErrProductMissing().DebugKV("sku", "SKU-123", "warehouse", "msk-1")
	// message → "товар SKU-123 не найден на складе msk-1"

	_ = ErrTableOp(err).DebugKV("op", "insert", "table", "orders")
	// message → "failed to insert in orders"

	// Шаблон инлайн — MsgTpl после данных
	_ = bang.Conflict("orders.stock").
		DebugKV("sku", "SKU-123", "qty", 0).
		MsgTpl("товар {sku} закончился")
	// message → "товар SKU-123 закончился"
}

// ═══════════════════════════════════════════════════════════════════════════
//  6. VALIDATION
// ═══════════════════════════════════════════════════════════════════════════

func Example_validation() {
	// Простая валидация — 3 вызова
	_ = bang.Validation("users.register").
		Field("email", "email_format", "Введите корректный email.").
		Field("password", "min_length", "Пароль — не менее 8 символов.")

	// Комбинирование валидаций
	err1 := bang.Validation("passport").
		Field("passports.0.issueDate", "date_format", "Исправьте дату выдачи.")
	err2 := bang.Validation("address").
		Field("address.city", "required", "Укажите город")

	_ = bang.CombineValidation(err1, err2)
	// → объединённый массив из 2 полей
}

// ═══════════════════════════════════════════════════════════════════════════
//  7. DB / REDIS ADAPTERS
// ═══════════════════════════════════════════════════════════════════════════

func Example_adapters() {
	var err error

	// WrapDBError: auto-class по тексту ошибки + код + debug
	_ = bang.WrapDBError(err).Code("users.repo.get_by_id").DebugKV("id", "u1")

	// Через Module — ещё короче
	_ = ordersRepo.WrapDBError(err, "get_by_id").DebugKV("id", "ord-1")

	// WrapRedisError
	_ = bang.WrapRedisError(err).Code("cache.session.get").DebugKV("sid", "s-123")
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

var ErrPaymentDeclined = bang.Define(ClassPaymentDeclined, "billing.charge_declined")

func Example_customClass() {
	_ = ErrPaymentDeclined().DebugKV("card", "***1234", "reason", "insufficient_funds")
}

// ═══════════════════════════════════════════════════════════════════════════
//  9. HTTP HANDLER
// ═══════════════════════════════════════════════════════════════════════════

func Example_httpHandler() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		err := doSomething(r.Context())
		if err != nil {
			// Лог — все данные
			bang.SlogError(slog.Default(), "request failed", err)

			// HTTP — safe only (debug для dev/admin)
			bang.WriteHTTP(w, err, bang.HTTPResponseOptions{
				Instance: r.URL.Path,
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
	var err error = ErrUserNotFound()

	// Через фабрику
	_ = ErrUserNotFound.Is(err) // true

	// По классу
	_ = bang.HasClass(err, bang.ClassNotFound) // true

	// По коду
	_ = bang.HasCode(err, "users.not_found") // true

	// IsValidation
	_ = bang.IsValidation(err) // false

	// Извлечение *Error
	if e, ok := bang.AsBang(err); ok {
		_ = e.GetFile() // "/app/users/service.go"
		_ = e.GetLine() // 42
	}

	// Типизированные данные
	if data, ok := bang.GetData[OrderData](err); ok {
		_ = data.OrderID
	}
}

func doSomething(_ context.Context) error {
	return fmt.Errorf("example")
}
