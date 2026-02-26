package bangval

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/smdmitry/bang"
)

const (
	bindValidationCode = "request.validation"
	bindJSONCode       = "request.bind"
)

type SliceValidationError []error

// TODO: костыль с Translator, придумать что-то другое
var Translator ut.Translator

func WrapValidationErr(err error) *bang.Error {
	if err == nil {
		return nil
	}
	if bangErr, ok := bang.AsBang(err); ok {
		return bangErr
	}

	fields := collectValidationErrors(err)
	if len(fields) > 0 {
		e := bang.Validation().Code(bindValidationCode).
			Msg("Некорректные параметры запроса").
			Wrap(err)

		for _, fe := range fields {
			if Translator != nil {
				e.Field(validationFieldKey(fe), fe.Tag(), fe.Translate(Translator))
			} else {
				e.Field(validationFieldKey(fe), fe.Tag(), validationReason(fe))
			}

		}
		return e
	}

	return bindPayloadErr(err)
}

func collectValidationErrors(err error) []validator.FieldError {
	if err == nil {
		return nil
	}

	out := make([]validator.FieldError, 0, 4)
	collectValidationErrorsInto(err, 0, &out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectValidationErrorsInto(err error, depth int, out *[]validator.FieldError) {
	if err == nil {
		return
	}
	if depth > 64 {
		return
	}

	if ves, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ves {
			*out = append(*out, fe)
		}
		return
	}

	for _, nested := range validationErrorSlice(err) {
		collectValidationErrorsInto(nested, depth+1, out)
	}

	type unwrapMany interface{ Unwrap() []error }
	if joined, ok := err.(unwrapMany); ok {
		for _, nested := range joined.Unwrap() {
			collectValidationErrorsInto(nested, depth+1, out)
		}
		return
	}

	collectValidationErrorsInto(errors.Unwrap(err), depth+1, out)
}

func validationErrorSlice(err error) []error {
	v := reflect.ValueOf(err)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil
	}

	out := make([]error, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if !item.IsValid() || !item.CanInterface() {
			continue
		}
		if (item.Kind() == reflect.Interface || item.Kind() == reflect.Pointer) && item.IsNil() {
			continue
		}
		if nested, ok := item.Interface().(error); ok && nested != nil {
			out = append(out, nested)
		}
	}
	return out
}

func validationFieldKey(fe validator.FieldError) string {
	key := strings.TrimSpace(fe.Namespace())
	if key == "" {
		key = strings.TrimSpace(fe.Field())
	}
	if idx := strings.IndexByte(key, '.'); idx >= 0 {
		key = key[idx+1:]
	}
	key = normalizeFieldPath(key)
	if key != "" {
		return key
	}
	return "body"
}

func validationReason(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "Поле обязательно"
	case "email":
		return "Некорректный email"
	case "uuid", "uuid4":
		return "Некорректный UUID"
	case "oneof":
		return fmt.Sprintf("Допустимые значения: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "len":
		return fmt.Sprintf("Длина должна быть %s", fe.Param())
	case "min":
		return fmt.Sprintf("Минимальное значение: %s", fe.Param())
	case "max":
		return fmt.Sprintf("Максимальное значение: %s", fe.Param())
	case "gte":
		return fmt.Sprintf("Значение должно быть не меньше %s", fe.Param())
	case "lte":
		return fmt.Sprintf("Значение должно быть не больше %s", fe.Param())
	case "msg_in_param":
		return fmt.Sprintf("%s", fe.Param())
	default:
		return "Некорректное значение"
	}
}

func bindPayloadErr(err error) *bang.Error {
	e := bang.Validation().Code(bindJSONCode).
		Msg("Некорректное тело запроса").
		Wrap(err)

	if errors.Is(err, io.EOF) {
		return e.Field("body", "required", "Тело запроса не должно быть пустым")
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		return e.Field("body", "required", "Неожиданный конец тела запроса")
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return e.Field("body", "json_syntax", "Некорректный JSON")
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		key := normalizeFieldPath(typeErr.Field)
		if key == "" {
			key = "body"
		}
		reason := "Некорректный тип значения"
		if typeErr.Type != nil {
			reason = fmt.Sprintf("Ожидается тип %s", readableType(typeErr.Type))
		}
		return e.Field(key, "type", reason)
	}

	if field, ok := unknownField(err); ok {
		return e.Field(field, "unknown_field", "Неизвестное поле")
	}

	return e.Field("body", "bind", "Не удалось разобрать тело запроса")
}

func unknownField(err error) (string, bool) {
	const prefix = `json: unknown field "`
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) || !strings.HasSuffix(msg, `"`) {
		return "", false
	}
	field := msg[len(prefix) : len(msg)-1]
	field = normalizeFieldPath(field)
	if field == "" {
		field = "body"
	}
	return field, true
}

func normalizeFieldPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "[", ".")
	path = strings.ReplaceAll(path, "]", "")

	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if isDigits(part) {
			out = append(out, part)
			continue
		}
		out = append(out, lowerFirst(part))
	}
	return strings.Join(out, ".")
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func readableType(t reflect.Type) string {
	if t == nil {
		return "unknown"
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.String()
}
