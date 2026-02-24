# bang — Structured Module Errors for Go

Гибкая библиотека ошибок с классификацией, уникальными кодами, typed data, шаблонами сообщений, RFC 7807 HTTP-ответами и структурированным логированием.

## Установка

```bash
go get github.com/user/bang
```

## Быстрый старт

```go
// 1 вызов
bang.NotFound("users.get_by_id")
bang.DatabaseError(err)
bang.SerializationError(err, "config.parse")

// 2 вызова
bang.NotFound(err, "users.get").Msgf("user %s not found", id)
bang.DatabaseError(err, "users.create").DebugKV("email", email)

// Фабрика
var ErrUserNotFound = bang.Define(bang.ClassNotFound, "users.not_found")
ErrUserNotFound()
ErrUserNotFound(err).DebugKV("id", id)

// Модуль
var Mod = bang.NewModule("orders.repo")
Mod.NotFound(err, "get_by_id")          // code → orders.repo.get_by_id
Mod.WrapDBError(err, "create")           // auto-class + code → orders.repo.create
```

## Конструкторы — гибкие аргументы

Все конструкторы принимают `(args ...any)` — любую комбинацию `error` и `string`:

```go
bang.NotFound()                        // дефолтный code
bang.NotFound("users.get")             // код
bang.NotFound(err)                     // wrap
bang.NotFound(err, "users.get")        // wrap + код
```

## Module — общий префикс

```go
var Mod = bang.NewModule("orders.repo")

Mod.NotFound(err, "get_by_id")          // code → "orders.repo.get_by_id"
Mod.DatabaseError(err)                   // code → "orders.repo"
Mod.WrapDBError(err, "update")           // auto-class + code → "orders.repo.update"

// Фабрики тоже через Module
var ErrNotFound = Mod.Define(bang.ClassNotFound, "not_found")  // code → "orders.repo.not_found"
```

## Define — callable фабрики

```go
var ErrUserNotFound = bang.Define(bang.ClassNotFound, "users.not_found")
var ErrUserExists   = bang.Define(bang.ClassConflict, "users.exists",
    bang.WithMessage("Пользователь уже зарегистрирован."),
)

ErrUserNotFound()                           // new
ErrUserNotFound(err)                        // wrap
ErrUserNotFound().Msgf("user %s", id)       // + message
ErrUserNotFound.Is(err)                     // check
```

## DefineTyped — с типизированными данными

```go
type OrderData struct {
    OrderID string `json:"orderId" expose:"true"`   // safe
    Status  string `json:"status"  expose:"true"`   // safe
    Query   string `json:"query"`                    // unsafe
}

var ErrOrderFailed = bang.DefineTyped[OrderData](bang.ClassDatabaseError, "orders.failed")

ErrOrderFailed().Data(OrderData{OrderID: id})
ErrOrderFailed(err).Data(OrderData{OrderID: id, Query: q})

// Типизированное извлечение
data, ok := ErrOrderFailed.GetData(err)
```

## MsgTpl — шаблоны сообщений с подстановками

Плейсхолдеры `{key}` резолвятся из Data и Debug автоматически:

```go
// В Define — резолвится при вызове .Data() / .DebugKV()
var ErrProductMissing = bang.Define(bang.ClassNotFound, "products.missing",
    bang.WithMsgTpl("товар {sku} не найден на складе {warehouse}"),
)
ErrProductMissing().DebugKV("sku", "X1", "warehouse", "msk-1")
// message → "товар X1 не найден на складе msk-1"

// Инлайн
bang.Conflict("orders.stock").
    DebugKV("sku", "X1", "qty", 0).
    MsgTpl("товар {sku} закончился")
```

## Validation

```go
bang.Validation("users.register").
    Field("email", "email_format", "Введите корректный email.").
    Field("password", "min_length", "Пароль — не менее 8 символов.")

// Комбинирование
combined := bang.CombineValidation(errPassport, errAddress)
```

## DB / Redis адаптеры

```go
bang.WrapDBError(err).Code("users.get").DebugKV("id", id)
// auto: no rows → NotFound, duplicate → DuplicateKey, foreign key → ForeignKeyViolation

bang.WrapRedisError(err).Code("cache.get").DebugKV("key", key)
// auto: redis:nil → NotFound, connection → NetworkError

// Через Module
Mod.WrapDBError(err, "get_by_id").DebugKV("id", id)
```

## HTTP-ответ (RFC 7807)

```go
bang.WriteHTTP(w, err, bang.HTTPResponseOptions{
    Instance:  r.URL.Path,
    ShowDebug: isDev || isAdmin,
})
```

## Логирование

```go
slog.Error("op failed", bang.SlogAttrs(err)...)   // slog attrs
bang.SlogError(logger, "op failed", err)            // helper
bang.ToJSON(err)                                    // JSON string
```

## Проверка ошибок

```go
ErrUserNotFound.Is(err)                              // по фабрике
bang.HasClass(err, bang.ClassNotFound)             // по классу
bang.HasCode(err, "users.not_found")                // по коду
bang.IsValidation(err)                              // валидация?
data, ok := bang.GetData[OrderData](err)            // typed data
data, ok := ErrOrderFailed.GetData(err)              // typed через фабрику
e, ok := bang.AsBang(err)                          // *Error
```

## Custom Class

```go
const ClassPaymentDeclined bang.Class = "payment_declined"

func init() {
    bang.RegisterClass(ClassPaymentDeclined, bang.ClassDef{
        DefaultCode: "payment_declined",
        Title:       "Payment Declined",
        Message:     "Платёж отклонён банком.",
        HTTPStatus:  402,
    })
}
```

## Тег `expose:"true"`

Поля с `expose:"true"` — safe (видны пользователю в HTTP). Остальные — unsafe (только логи/debug).

## Структура файлов

```
bang/
├── doc.go           — пакетная документация
├── class.go         — Class, константы, ClassDef, RegisterClass
├── error.go         — Error struct, builder, MsgTpl, DebugKV, accessors
├── constructors.go  — NotFound(), DatabaseError(), New(), WrapUnknown()
├── descriptor.go    — Define(), DefineTyped(), ErrFactory, TypedErrFactory
├── module.go        — Module, NewModule(), ModuleDefineTyped()
├── validation.go    — Validation(), Field(), CombineValidation()
├── expose.go        — FilterSafe(), ToMap()
├── adapters.go      — WrapDBError(), WrapRedisError()
├── http.go          — ToHTTP(), WriteHTTP(), ProblemDetail
├── log.go           — ToLogRecord(), ToJSON(), SlogAttrs(), SlogError()
└── bangval/        — интеграция с go-playground/validator/v10
    ├── adapter.go       — Validator, Struct(), конвертация путей
    ├── messages_ru.go   — русские сообщения для всех стандартных тегов
    └── checker.go       — Checker, NewCheckFn, кастомные валидаторы
```

## bangval — валидация с русскими сообщениями

### Базовая struct-валидация тегами

```go
v := bangval.New()
err := v.Struct("users.register", req) // → *bang.Error или nil
```

Все стандартные теги validator/v10 (`required`, `email`, `min`, `max`, `oneof`, и т.д.)
автоматически возвращают сообщения на русском. Пути к полям строятся из json тегов:
`passports.0.issueDate` вместо `Passports[0].IssueDate`.

### Кастомные сообщения для конкретных полей

```go
v := bangval.New(bangval.WithFieldMessages(map[string]string{
    "CreateUserReq.Email.required":  "Мы не сможем отправить подтверждение без email",
    "CreateUserReq.Password.min":    "Пароль слишком короткий, нужно минимум 8 символов",
}))
```

### Кастомные валидаторы с разными rule/reason

Определите переиспользуемый валидатор с `NewCheckFn`:

```go
var checkPassportSerial = bangval.NewCheckFn(func(v string) *bangval.FieldError {
    if len(v) != 4 {
        return bangval.Fail("serial_length", "Серия паспорта должна содержать 4 цифры")
    }
    if !allDigits(v) {
        return bangval.Fail("serial_digits", "Серия паспорта должна состоять только из цифр")
    }
    return nil
})
```

Используйте через `Checker`:

```go
c := bangval.NewChecker("registration.passports")
for i, p := range req.Passports {
    prefix := fmt.Sprintf("passports.%d", i)
    c.Check(prefix+".serial", checkPassportSerial.Bind(p.Serial))
    c.Check(prefix+".issueDate", checkPassportDate.Bind(p.IssueDate))
}
return c.Err() // nil или *bang.Error
```

Инлайн-проверки (для одноразовых):

```go
c.Check("endDate", func() *bangval.FieldError {
    if endDate < startDate {
        return bangval.Fail("date_range", "Дата окончания не может быть раньше даты начала")
    }
    return nil
})
```

### Комбинирование struct-валидации и кастомных проверок

```go
err := bang.CombineValidation(
    v.Struct("registration.create", req),    // struct теги
    validatePassports(req.Passports),         // кастомные проверки
)
```
