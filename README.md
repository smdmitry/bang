# bang — Structured Errors for Go

Гибкая библиотека ошибок с классификацией, уникальными кодами, typed data, шаблонами сообщений, RFC 7807 HTTP-ответами и структурированным логированием.

## Установка

```bash
go get github.com/smdmitry/bang
```

## Философия

- Каждое место в коде, где кидается ошибка, уникально (свой Code) — для логов и Prometheus
- Минимум параметров, короткие вызовы
- Хочешь — используй `bang` напрямую, хочешь — регистрируй коды и фабрики
- Данные разделены на Safe (видны юзеру) и Debug (только логи)
- RFC 7807 из коробки

## Быстрый старт

```go
// 1 строка — самый простой случай
return bang.NotFound()

// Wrap + код + debug
return bang.Database().Wrap(err).Code("users.create").Debug("email", email)

// Полный пример
return bang.NotFound().Wrap(err).
    Code("users.get_by_id").
    Safe("user_id", id).
    Debug("query", sql).
    Msg("Пользователь не найден")
```

## Конструкторы

Каждый конструктор возвращает `*Error` с нужным классом. Дальше — fluent builder:

```go
bang.NotFound()
bang.Database()
bang.Conflict()
bang.Unauthorized()
bang.Forbidden()
bang.InternalServer()
bang.BadRequest()
bang.TooManyRequests()
bang.SerializationError()
bang.Unexpected()
// ... и другие
```

## Builder-методы

```go
bang.NotFound().
    Wrap(err).                              // обернуть другую ошибку
    Code("module.submodule.error_descr").   // уникальный код
    Title("Переопределить заголовок").       // редко нужно
    Msg("Сообщение для юзера").             // текст ошибки
    Msgf("User %s not found", id).          // форматированное сообщение
    Msg("User {{.name}} not found").        // шаблон с подстановками
    Safe("key", "value").                   // видимые данные (k-v пары)
    SafeMap(bang.Map{"key": "value"}).       // видимые данные (map)
    Debug("key", "value").                  // unsafe данные (k-v пары)
    DebugMap(bang.Map{"key": "value"}).      // unsafe данные (map)
    Data(TypedStruct{...}).                 // типизированные данные
    WithStack().                            // захватить стек
    HTTPStatus(418)                         // переопределить HTTP код
```

## Регистрация кодов

### Простая регистрация сообщений

```go
bang.MustRegisterMessages(map[string]string{
    "task.not_found": "Task '{{.key}}' not found",
})

// Использование
bang.New("task.not_found").Safe("key", taskID)
```

### Полная регистрация кода

```go
bang.MustRegisterCodes(map[string]bang.CodeCfg{
    "task.not_found": {
        Class: bang.ClassNotFound,
        Title: "Task Not Found",
        Msg:   "Task '{{.key}}' not found",
        // HTTPCode: 404, // можно переопределить
    },
})
```

### Factory — фабрика ошибок

```go
var TaskNotFound = bang.Factory("task.not_found")

// Использование
return TaskNotFound().Debug("id", id)
return TaskNotFound().Wrap(err).Safe("key", "value")

// Проверка
if TaskNotFound.Is(err) { ... }
```

### Register — builder-стиль регистрации

```go
var TaskNotFound = bang.Register("task.not_found").
    Class(bang.ClassNotFound).
    Title("Task Not Found").
    Message("Task '{{.key}}' not found").
    New()

// Использование
return TaskNotFound().Safe("key", id).Debug("query", sql)
```

### Типизированная фабрика

```go
type TaskNotFoundData struct {
    TaskID string `json:"task_id" expose:"true"`
    Owner  string `json:"owner,omitempty"`
}

var TaskNotFound = bang.RegisterTyped[TaskNotFoundData]("task.not_found").
    Class(bang.ClassNotFound).
    Message("Task '{{.task_id}}' not found").
    New()

// Использование — данные обязательны
return TaskNotFound(TaskNotFoundData{TaskID: "t-1", Owner: "admin"})

// Извлечение
data, ok := TaskNotFound.GetData(err)

// Альтернатива (старый стиль) — тоже поддерживается:
// bang.Typed[TaskNotFoundData](bang.Register("task.not_found").Class(...).Message(...))
```

## Prefix factory — общий префикс

```go
var repoerr = bang.Prefix("orders.repo")
var repoerrNotFound = bang.Prefix("orders.repo").Class(bang.ClassNotFound)

repoerr.NotFound()                          // code → "orders.repo"
repoerr.Conflict().Code("archive_failed")   // code → "orders.repo.archive_failed"
repoerrNotFound.New()                       // class → not_found, code → "orders.repo"
repoerr.WrapDBError(err, "create")          // auto-class + code → "orders.repo.create"
repoerr.WrapRedisError(err, "cache")        // auto-class + code → "orders.repo.cache"
```

## Validation

```go
// Создание
bang.Validation().Code("users.register").
    Field("email", "email_format", "Введите корректный email.").
    Field("password", "min_length", "Пароль — не менее 8 символов.")

// Комбинирование
combined := bang.CombineValidation(errPassport, errAddress)
```

### bangval — интеграция с go-playground/validator

```go
v := bangval.New()
err := v.Struct("users.register", req) // → *bang.Error или nil

// Кастомные сообщения для конкретных полей
v := bangval.New(bangval.WithFieldMessages(map[string]string{
    "CreateUserReq.Email.required":  "Укажите email для подтверждения",
    "CreateUserReq.Password.min":    "Пароль должен быть минимум 8 символов",
}))

// Кастомные валидаторы
var checkPassportSerial = bangval.NewCheckFn(func(v string) *bangval.FieldError {
    if len(v) != 4 {
        return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
    }
    return nil
})

c := bangval.NewChecker("registration.passports")
c.Check("passports.0.serial", checkPassportSerial.Bind(p.Serial))
return c.Err()

// Комбинирование struct-валидации и кастомных проверок
err := bang.CombineValidation(
    v.Struct("registration.create", req),
    validatePassports(req.Passports),
)
```

## DB / Redis адаптеры

```go
// Auto-classification по тексту ошибки
bang.WrapDBError(err).Code("users.repo.get").Debug("id", id)
// no rows → NotFound, duplicate key → DuplicateKey, foreign key → ForeignKeyViolation

bang.WrapRedisError(err).Code("cache.get").Debug("key", key)
// redis:nil → NotFound, connection refused → NetworkError

// Через Prefix factory
repoerr.WrapDBError(err, "get_by_id").Debug("id", id)
```

## HTTP-ответ (RFC 7807)

```go
bang.WriteHTTP(w, err, bang.HTTPResponseOptions{
    Instance:  r.URL.Path,
    ShowDebug: isDev || isAdmin,
})
```

Формат ответа:
```json
{
    "type": "urn:error:not_found:users.get_by_id",
    "status": 404,
    "title": "Not Found",
    "detail": "Пользователь не найден",
    "instance": "/api/users/123",
    "errorType": "business",
    "details": {"user_id": "123"},
    "debug": {
        "data": {"user_id": "123", "query": "SELECT ..."},
        "file": "/app/users/repo.go",
        "line": 42,
        "stack": [...]
    }
}
```

Для validation:
```json
{
    "type": "urn:error:validation:users.register",
    "status": 422,
    "errorType": "validation",
    "errors": [
        {"key": "email", "reason": "Введите корректный email"},
        {"key": "password", "reason": "Пароль — не менее 8 символов"}
    ]
}
```

## Логирование

```go
slog.Error("op failed", bang.SlogAttrs(err)...)   // slog attrs
bang.SlogError(logger, "op failed", err)            // helper
bang.ToJSON(err)                                    // JSON string
```

## Проверка ошибок

```go
// По фабрике
TaskNotFound.Is(err)

// По классу
bang.HasClass(err, bang.ClassNotFound)

// По коду
bang.HasCode(err, "users.not_found")

// Валидация?
bang.IsValidation(err)

// errors.Is
errors.Is(err, bang.NotFound())

// Извлечение *Error
if e, ok := bang.AsBang(err); ok {
    _ = e.GetFile()
    _ = e.GetLine()
}

// Типизированные данные
if data, ok := bang.GetData[OrderData](err); ok {
    _ = data.OrderID
}
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

Поля с `expose:"true"` — safe (видны юзеру в HTTP). Остальные — unsafe (только логи/debug).

```go
type OrderData struct {
    OrderID string `json:"orderId" expose:"true"` // видно юзеру
    Query   string `json:"query"`                  // только в логах
}
```

## Настройки

```go
// Как обрабатывать незаполненные подстановки в шаблонах
bang.SetTemplateMissingKeyMode(bang.MissingKeyKeep)   // оставить {{.key}} (default)
bang.SetTemplateMissingKeyMode(bang.MissingKeyRemove)  // удалить
bang.SetTemplateMissingKeyMode(bang.MissingKeyError)   // ошибка
```

## banggin — Gin фреймворк

```go
r := gin.New()
r.Use(banggin.ErrorMiddleware())
r.Use(banggin.RecoveryMiddleware())

r.GET("/users/:id", func(c *gin.Context) {
    banggin.AbortError(c, bang.NotFound().Code("users.get"))
})
```

## Структура файлов

```
bang/
├── doc.go           — пакетная документация
├── class.go         — Class, константы, ClassDef, RegisterClass
├── error.go         — Error struct, builder, Safe/Debug/Data, accessors
├── constructors.go  — NotFound(), Database(), New(), WrapUnknown()
├── registry.go      — MustRegisterMessages, MustRegisterCodes, Factory, Register, RegisterTyped, Typed
├── validation.go    — Validation(), Field(), CombineValidation()
├── expose.go        — FilterSafe(), ToMap()
├── adapters.go      — WrapDBError(), WrapRedisError()
├── http.go          — ToHTTP(), WriteHTTP(), ProblemDetail
├── log.go           — ToLogRecord(), ToJSON(), SlogAttrs(), SlogError()
├── prefix.go        — Prefix factory
├── bangval/         — интеграция с go-playground/validator/v10
│   ├── validator.go     — Validator, Struct(), русские сообщения
│   └── checker.go       — Checker, NewCheckFn, Fail()
└── banggin/         — интеграция с Gin
    ├── banggin.go       — ShouldBindJSON, AbortError, RecoveryMiddleware
    ├── middleware.go     — ErrorMiddleware, WithOptionsProvider
    └── bind.go          — BindErr, конвертация ошибок
```
