package bang

import "net/http"

// Map is a convenience alias for untyped key-value data.
type Map = map[string]any

// Class represents a category of error. Each class has default code, title,
// message, and HTTP status mapping.
type Class string

const (
	ClassInternalServer  Class = "internal_server"
	ClassValidation      Class = "validation"
	ClassBadRequest      Class = "bad_request"
	ClassNotFound        Class = "not_found"
	ClassUnauthorized    Class = "unauthorized"
	ClassForbidden       Class = "forbidden"
	ClassConflict        Class = "conflict"
	ClassTooManyRequests Class = "too_many_requests"

	ClassDatabase Class = "database"

	ClassDuplicateKey        Class = "duplicate_key"
	ClassForeignKeyViolation Class = "foreign_key_violation"

	ClassExternalService Class = "external_service"
	ClassNetwork         Class = "network"
	ClassTimeout         Class = "timeout"

	ClassSerialization  Class = "serialization"
	ClassUnexpected     Class = "unexpected"
	ClassNotImplemented Class = "not_implemented"
	ClassLocked         Class = "locked"
)

// classMeta holds default metadata for each class.
type classMeta struct {
	DefaultCode string
	Title       string
	Message     string
	HTTPStatus  int
}

var classRegistry = map[Class]classMeta{
	ClassInternalServer:  {"internal_server", "Ошибка сервера", "На сервере произошла ошибка. Пожалуйста, попробуйте позже.", http.StatusInternalServerError},
	ClassValidation:      {"validation", "Ошибка проверки данных", "В запросе указаны некорректные данные.", http.StatusUnprocessableEntity},
	ClassBadRequest:      {"bad_request", "Некорректный запрос", "Запрос составлен неверно.", http.StatusBadRequest},
	ClassNotFound:        {"not_found", "Не найдено", "Запрашиваемый ресурс не найден.", http.StatusNotFound},
	ClassUnauthorized:    {"unauthorized", "Требуется авторизация", "Для выполнения операции необходимо войти в систему.", http.StatusUnauthorized},
	ClassForbidden:       {"forbidden", "Доступ запрещён", "У вас нет прав для выполнения этой операции.", http.StatusForbidden},
	ClassConflict:        {"conflict", "Конфликт данных", "Операция не может быть выполнена из-за текущего состояния данных.", http.StatusConflict},
	ClassTooManyRequests: {"too_many_requests", "Слишком много запросов", "Превышено допустимое количество запросов. Попробуйте позже.", http.StatusTooManyRequests},

	ClassDatabase:            {"database", "Ошибка базы данных", "Произошла ошибка при работе с базой данных.", http.StatusInternalServerError},
	ClassDuplicateKey:        {"duplicate_key", "Данные уже существуют", "Запись с такими данными уже существует.", http.StatusConflict},
	ClassForeignKeyViolation: {"foreign_key_violation", "Связанная запись не найдена", "Указанная связанная запись не существует.", http.StatusUnprocessableEntity},

	ClassExternalService: {"external_service", "Ошибка внешнего сервиса", "Внешний сервис временно недоступен или вернул ошибку.", http.StatusBadGateway},
	ClassNetwork:         {"network", "Ошибка сети", "Произошла сетевая ошибка. Попробуйте позже.", http.StatusBadGateway},
	ClassTimeout:         {"timeout", "Превышено время ожидания", "Время ожидания операции истекло. Попробуйте позже.", http.StatusGatewayTimeout},

	ClassSerialization: {"serialization", "Ошибка обработки данных", "Не удалось обработать данные.", http.StatusInternalServerError},
	ClassUnexpected:    {"unexpected", "Непредвиденная ошибка", "Произошла непредвиденная ошибка. Попробуйте позже.", http.StatusInternalServerError},

	ClassNotImplemented: {"not_implemented", "Функция недоступна", "Данная функция пока не поддерживается.", http.StatusNotImplemented},
	ClassLocked:         {"locked", "Ресурс заблокирован", "Запрашиваемый ресурс временно заблокирован.", http.StatusLocked},
}

// ClassDef describes a custom error class for registration.
type ClassDef struct {
	DefaultCode string
	Title       string
	Message     string
	HTTPStatus  int
}

// RegisterClass registers a custom error class.
func RegisterClass(class Class, def ClassDef) {
	classRegistry[class] = classMeta{
		DefaultCode: def.DefaultCode,
		Title:       def.Title,
		Message:     def.Message,
		HTTPStatus:  def.HTTPStatus,
	}
}

func getClassMeta(class Class) classMeta {
	if m, ok := classRegistry[class]; ok {
		return m
	}
	return classRegistry[ClassUnexpected]
}

// HTTPStatus returns the HTTP status code for this class.
func (c Class) HTTPStatus() int { return getClassMeta(c).HTTPStatus }

// String implements fmt.Stringer.
func (c Class) String() string { return string(c) }
