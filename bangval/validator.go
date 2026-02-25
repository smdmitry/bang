// Package bangval integrates go-playground/validator/v10 with bang,
// providing Russian human-readable messages and custom multi-condition validators.
//
// # Standard validation via struct tags
//
//	v := bangval.New()
//	err := v.Struct("users.register", req)
//	// → *bang.Error{ClassValidation} with Russian messages, or nil
//
// # Custom validators
//
//	c := bangval.NewChecker("users.register")
//	c.Check("email", checkEmail.Bind(req.Email))
//	err := c.Err()
//
// # Combine struct + custom
//
//	err := bang.CombineValidation(
//	    v.Struct("users.register", req),
//	    validatePassports(req.Passports),
//	)
package bangval

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/smdmitry/bang"
)

// FieldInfo contains information about a failed field, passed to MessageFunc.
type FieldInfo struct {
	Field string       // field name (json tag or Go name)
	Tag   string       // validation tag that failed
	Param string       // tag parameter (e.g., "8" for min=8)
	Value any          // actual field value
	Kind  reflect.Kind // field kind
	Type  reflect.Type // field type
}

// MessageFunc generates a human-readable error message for a validation tag.
type MessageFunc func(fe FieldInfo) string

// Validator wraps go-playground/validator/v10 with bang integration.
type Validator struct {
	v             *validator.Validate
	messages      map[string]MessageFunc // tag → message function
	fieldMessages map[string]string      // "Struct.Field.tag" → custom message
}

// Option configures a Validator.
type Option func(*Validator)

// New creates a Validator with Russian messages by default.
func New(opts ...Option) *Validator {
	v := &Validator{
		v:             validator.New(validator.WithRequiredStructEnabled()),
		messages:      defaultMessagesRu(),
		fieldMessages: make(map[string]string),
	}
	v.v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("json")
		if name == "" || name == "-" {
			return fld.Name
		}
		if i := strings.IndexByte(name, ','); i != -1 {
			name = name[:i]
		}
		return name
	})
	for _, o := range opts {
		o(v)
	}
	return v
}

// WithMessages overrides or adds message functions for specific tags.
func WithMessages(msgs map[string]MessageFunc) Option {
	return func(v *Validator) {
		for tag, fn := range msgs {
			v.messages[tag] = fn
		}
	}
}

// WithFieldMessages sets custom messages for specific struct.field.tag combinations.
func WithFieldMessages(msgs map[string]string) Option {
	return func(v *Validator) {
		for k, msg := range msgs {
			v.fieldMessages[k] = msg
		}
	}
}

// Inner returns the underlying *validator.Validate for custom registrations.
func (val *Validator) Inner() *validator.Validate {
	return val.v
}

// Struct validates a struct and returns a *bang.Error with ClassValidation,
// or nil if validation passes.
func (val *Validator) Struct(code string, obj any) *bang.Error {
	err := val.v.Struct(obj)
	if err == nil {
		return nil
	}

	ves, ok := err.(validator.ValidationErrors)
	if !ok {
		return bang.InternalServer().Wrap(err).Code("bangval.invalid_struct")
	}

	if len(ves) == 0 {
		return nil
	}

	structType := reflect.TypeOf(obj)
	for structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	e := bang.Validation().Code(code)
	for _, fe := range ves {
		key := buildFieldPath(fe, structType)
		rule := fe.Tag()
		reason := val.resolveMessage(fe, structType)
		e.Field(key, rule, reason)
	}
	return e
}

func (val *Validator) resolveMessage(fe validator.FieldError, structType reflect.Type) string {
	fieldKey := structType.Name() + "." + fe.StructField() + "." + fe.Tag()
	if msg, ok := val.fieldMessages[fieldKey]; ok {
		return msg
	}

	info := FieldInfo{
		Field: fe.Field(),
		Tag:   fe.Tag(),
		Param: fe.Param(),
		Value: fe.Value(),
		Kind:  fe.Kind(),
		Type:  fe.Type(),
	}
	if fn, ok := val.messages[fe.Tag()]; ok {
		return fn(info)
	}

	return "Значение некорректно"
}

// ─── Field path resolution ──────────────────────────────────────────────────

func buildFieldPath(fe validator.FieldError, rootType reflect.Type) string {
	ns := fe.StructNamespace()
	if rootType.Name() != "" {
		prefix := rootType.Name() + "."
		if strings.HasPrefix(ns, prefix) {
			ns = ns[len(prefix):]
		}
	}
	return resolveJSONPath(ns, rootType)
}

func resolveJSONPath(goPath string, rootType reflect.Type) string {
	segments := splitNamespace(goPath)
	var result []string
	currentType := rootType

	for _, seg := range segments {
		if seg.index != "" {
			result = append(result, seg.index)
			if currentType != nil {
				if currentType.Kind() == reflect.Slice || currentType.Kind() == reflect.Array {
					currentType = currentType.Elem()
					for currentType.Kind() == reflect.Ptr {
						currentType = currentType.Elem()
					}
				}
			}
			continue
		}

		jsonName := seg.name
		if currentType != nil && currentType.Kind() == reflect.Struct {
			if f, ok := currentType.FieldByName(seg.name); ok {
				jsonName = jsonTagName(f)
				ft := f.Type
				for ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				switch ft.Kind() {
				case reflect.Slice, reflect.Array, reflect.Struct:
					currentType = ft
				default:
					currentType = nil
				}
			} else {
				currentType = nil
			}
		} else {
			jsonName = lcFirst(seg.name)
			currentType = nil
		}
		result = append(result, jsonName)
	}

	return strings.Join(result, ".")
}

type pathSegment struct {
	name  string
	index string
}

func splitNamespace(ns string) []pathSegment {
	var segments []pathSegment
	for ns != "" {
		if ns[0] == '[' {
			end := strings.IndexByte(ns, ']')
			if end == -1 {
				break
			}
			segments = append(segments, pathSegment{index: ns[1:end]})
			ns = ns[end+1:]
			if len(ns) > 0 && ns[0] == '.' {
				ns = ns[1:]
			}
			continue
		}

		nextDot := strings.IndexByte(ns, '.')
		nextBracket := strings.IndexByte(ns, '[')

		end := len(ns)
		if nextDot >= 0 && (nextBracket < 0 || nextDot < nextBracket) {
			end = nextDot
		} else if nextBracket >= 0 {
			end = nextBracket
		}

		segments = append(segments, pathSegment{name: ns[:end]})
		ns = ns[end:]
		if len(ns) > 0 && ns[0] == '.' {
			ns = ns[1:]
		}
	}
	return segments
}

func jsonTagName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return lcFirst(f.Name)
	}
	if i := strings.IndexByte(tag, ','); i != -1 {
		tag = tag[:i]
	}
	return tag
}

func lcFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// ─── Default Russian messages ───────────────────────────────────────────────

func constMsg(msg string) MessageFunc {
	return func(FieldInfo) string { return msg }
}

func defaultMessagesRu() map[string]MessageFunc {
	return map[string]MessageFunc{
		"required":             constMsg("Поле обязательно для заполнения"),
		"required_if":          constMsg("Поле обязательно для заполнения"),
		"required_unless":      constMsg("Поле обязательно для заполнения"),
		"required_with":        constMsg("Поле обязательно для заполнения"),
		"required_with_all":    constMsg("Поле обязательно для заполнения"),
		"required_without":     constMsg("Поле обязательно для заполнения"),
		"required_without_all": constMsg("Поле обязательно для заполнения"),

		"min": func(fe FieldInfo) string {
			switch fe.Kind {
			case reflect.String:
				return fmt.Sprintf("Минимальная длина — %s символов", fe.Param)
			case reflect.Slice, reflect.Array, reflect.Map:
				return fmt.Sprintf("Минимальное количество элементов — %s", fe.Param)
			default:
				return fmt.Sprintf("Минимальное значение — %s", fe.Param)
			}
		},
		"max": func(fe FieldInfo) string {
			switch fe.Kind {
			case reflect.String:
				return fmt.Sprintf("Максимальная длина — %s символов", fe.Param)
			case reflect.Slice, reflect.Array, reflect.Map:
				return fmt.Sprintf("Максимальное количество элементов — %s", fe.Param)
			default:
				return fmt.Sprintf("Максимальное значение — %s", fe.Param)
			}
		},
		"len": func(fe FieldInfo) string {
			switch fe.Kind {
			case reflect.String:
				return fmt.Sprintf("Длина должна быть %s символов", fe.Param)
			case reflect.Slice, reflect.Array, reflect.Map:
				return fmt.Sprintf("Количество элементов должно быть %s", fe.Param)
			default:
				return fmt.Sprintf("Значение должно быть равно %s", fe.Param)
			}
		},
		"eq": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно быть равно %s", fe.Param)
		},
		"ne": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение не должно быть равно %s", fe.Param)
		},
		"gt": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно быть больше %s", fe.Param)
		},
		"gte": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно быть не менее %s", fe.Param)
		},
		"lt": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно быть меньше %s", fe.Param)
		},
		"lte": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно быть не более %s", fe.Param)
		},

		"eqfield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно совпадать с полем %s", fe.Param)
		},
		"nefield": func(fe FieldInfo) string {
			return fmt.Sprintf("Не должно совпадать с полем %s", fe.Param)
		},
		"gtfield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно быть больше поля %s", fe.Param)
		},
		"gtefield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно быть не менее поля %s", fe.Param)
		},
		"ltfield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно быть меньше поля %s", fe.Param)
		},
		"ltefield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно быть не более поля %s", fe.Param)
		},
		"eqcsfield": func(fe FieldInfo) string {
			return fmt.Sprintf("Должно совпадать с полем %s", fe.Param)
		},
		"necsfield": func(fe FieldInfo) string {
			return fmt.Sprintf("Не должно совпадать с полем %s", fe.Param)
		},

		"email":       constMsg("Введите корректный email-адрес"),
		"url":         constMsg("Введите корректный URL"),
		"uri":         constMsg("Введите корректный URI"),
		"http_url":    constMsg("Введите корректный HTTP URL"),
		"uuid":        constMsg("Некорректный формат UUID"),
		"uuid3":       constMsg("Некорректный формат UUID v3"),
		"uuid4":       constMsg("Некорректный формат UUID v4"),
		"uuid5":       constMsg("Некорректный формат UUID v5"),
		"ulid":        constMsg("Некорректный формат ULID"),
		"ip":          constMsg("Введите корректный IP-адрес"),
		"ipv4":        constMsg("Введите корректный IPv4-адрес"),
		"ipv6":        constMsg("Введите корректный IPv6-адрес"),
		"cidr":        constMsg("Введите корректный CIDR"),
		"mac":         constMsg("Введите корректный MAC-адрес"),
		"e164":        constMsg("Введите номер телефона в международном формате (например, +79001234567)"),
		"isbn":        constMsg("Некорректный формат ISBN"),
		"isbn10":      constMsg("Некорректный формат ISBN-10"),
		"isbn13":      constMsg("Некорректный формат ISBN-13"),
		"base64":      constMsg("Некорректный формат Base64"),
		"hexadecimal": constMsg("Значение должно быть в шестнадцатеричном формате"),
		"hex_color":   constMsg("Введите корректный HEX-цвет (например, #FF5500)"),
		"rgb":         constMsg("Введите корректный RGB-цвет"),
		"rgba":        constMsg("Введите корректный RGBA-цвет"),
		"hsl":         constMsg("Введите корректный HSL-цвет"),
		"hsla":        constMsg("Введите корректный HSLA-цвет"),
		"json":        constMsg("Некорректный формат JSON"),
		"jwt":         constMsg("Некорректный формат JWT"),
		"html":        constMsg("Некорректный формат HTML"),

		"alpha":           constMsg("Допускаются только буквы"),
		"alphanum":        constMsg("Допускаются только буквы и цифры"),
		"alphaunicode":    constMsg("Допускаются только буквы"),
		"alphanumunicode": constMsg("Допускаются только буквы и цифры"),
		"numeric":         constMsg("Значение должно быть числом"),
		"number":          constMsg("Значение должно быть числом"),
		"boolean":         constMsg("Значение должно быть true или false"),
		"lowercase":       constMsg("Допускаются только строчные буквы"),
		"uppercase":       constMsg("Допускаются только заглавные буквы"),
		"ascii":           constMsg("Допускаются только ASCII-символы"),
		"printascii":      constMsg("Допускаются только печатные ASCII-символы"),

		"contains": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно содержать «%s»", fe.Param)
		},
		"containsany": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно содержать хотя бы один из символов: %s", fe.Param)
		},
		"excludes": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение не должно содержать «%s»", fe.Param)
		},
		"excludesall": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение не должно содержать символы: %s", fe.Param)
		},
		"startswith": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно начинаться с «%s»", fe.Param)
		},
		"endswith": func(fe FieldInfo) string {
			return fmt.Sprintf("Значение должно заканчиваться на «%s»", fe.Param)
		},

		"oneof": func(fe FieldInfo) string {
			return fmt.Sprintf("Допустимые значения: %s", fe.Param)
		},
		"excluded_with":    constMsg("Поле должно быть пустым"),
		"excluded_without": constMsg("Поле должно быть пустым"),

		"unique": constMsg("Значения не должны повторяться"),
		"dive":   constMsg("Один или несколько элементов некорректны"),

		"datetime": func(fe FieldInfo) string {
			if fe.Param != "" {
				return fmt.Sprintf("Некорректный формат даты (ожидается: %s)", fe.Param)
			}
			return "Некорректный формат даты"
		},

		"file":     constMsg("Укажите корректный путь к файлу"),
		"filepath": constMsg("Укажите корректный путь"),
		"image":    constMsg("Файл должен быть изображением"),

		"credit_card": constMsg("Введите корректный номер карты"),
		"regex":       constMsg("Значение не соответствует допустимому формату"),
	}
}
