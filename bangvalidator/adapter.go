package bangvalidator

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/smdmitry/bang"
)

const TagMsgInParam = "msg_in_param"

// Validator wraps go-playground/validator/v10 with bang integration.
type Validator struct {
	v             *validator.Validate
	messages      map[string]MessageFunc // tag → message function
	fieldMessages map[string]string      // "Struct.Field.tag" → custom message
}

// Option configures a Validator.
type Option func(*Validator)

// New creates a Validator with Russian messages by default.
//
//	v := bangval.New()
//	v := bangval.New(bangval.WithFieldMessages(map[string]string{
//	    "User.Email.required": "Пожалуйста, укажите ваш email",
//	}))
func New(opts ...Option) *Validator {
	v := &Validator{
		v:             validator.New(validator.WithRequiredStructEnabled()),
		messages:      defaultMessagesRu(),
		fieldMessages: make(map[string]string),
	}
	// Use json tags for field names in error messages.
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
// Key format: "StructName.FieldName.tag" (use Go struct names, not json names).
//
//	bangval.WithFieldMessages(map[string]string{
//	    "CreateUserReq.Email.required": "Мы не сможем отправить подтверждение без email",
//	    "CreateUserReq.Password.min":   "Пароль должен содержать минимум 8 символов",
//	})
func WithFieldMessages(msgs map[string]string) Option {
	return func(v *Validator) {
		for k, msg := range msgs {
			v.fieldMessages[k] = msg
		}
	}
}

// Inner returns the underlying *validator.Validate for registering
// custom validator/v10 tags, hooks, etc.
//
//	v.Inner().RegisterValidation("my_tag", myFunc)
func (val *Validator) Inner() *validator.Validate {
	return val.v
}

// ─── Struct validation ──────────────────────────────────────────────────────

// Struct validates a struct and returns a *bang.Error with ClassValidation,
// or nil if validation passes. All messages are in Russian by default.
//
//	err := v.Struct("users.register", req)
//	if err != nil {
//	    return err // *bang.Error with validation fields
//	}
func (val *Validator) Struct(code string, obj any) *bang.Error {
	err := val.v.Struct(obj)
	if err == nil {
		return nil
	}

	ves, ok := err.(validator.ValidationErrors)
	if !ok {
		// Not a ValidationErrors (e.g., InvalidValidationError) → internal error.
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

// ─── Message resolution ─────────────────────────────────────────────────────

func (val *Validator) resolveMessage(fe validator.FieldError, structType reflect.Type) string {
	// 1. Check per-field override: "StructName.GoFieldName.tag"
	fieldKey := structType.Name() + "." + fe.StructField() + "." + fe.Tag()
	if msg, ok := val.fieldMessages[fieldKey]; ok {
		return msg
	}

	// 2. Check per-tag message function.
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

	// 3. Fallback.
	return "Значение некорректно"
}

// ─── Field path ─────────────────────────────────────────────────────────────

// buildFieldPath converts validator namespace like "CreateReq.Passports[0].IssueDate"
// into a json-based dot path like "passports.0.issueDate".
func buildFieldPath(fe validator.FieldError, rootType reflect.Type) string {
	ns := fe.StructNamespace() // "StructName.Field[0].Sub"

	// Strip root struct name.
	if rootType.Name() != "" {
		prefix := rootType.Name() + "."
		if strings.HasPrefix(ns, prefix) {
			ns = ns[len(prefix):]
		}
	}

	// Split into segments, resolve json names.
	return resolveJSONPath(ns, rootType)
}

// resolveJSONPath walks the type hierarchy and converts Go field names to json tag names.
func resolveJSONPath(goPath string, rootType reflect.Type) string {
	segments := splitNamespace(goPath)
	var result []string
	currentType := rootType

	for _, seg := range segments {
		if seg.index != "" {
			// Array index segment like "0"
			result = append(result, seg.index)
			// Advance into array/slice element type.
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

		// Field name segment.
		jsonName := seg.name
		if currentType != nil && currentType.Kind() == reflect.Struct {
			if f, ok := currentType.FieldByName(seg.name); ok {
				jsonName = jsonTagName(f)
				ft := f.Type
				for ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
					currentType = ft
				} else if ft.Kind() == reflect.Struct {
					currentType = ft
				} else {
					currentType = nil
				}
			} else {
				currentType = nil
			}
		} else {
			// Can't resolve further — lowercase first letter as best effort.
			jsonName = lcFirst(seg.name)
			currentType = nil
		}
		result = append(result, jsonName)
	}

	return strings.Join(result, ".")
}

type pathSegment struct {
	name  string // Go field name (empty for index segments)
	index string // array index (empty for field segments)
}

// splitNamespace splits "Passports[0].IssueDate" into segments.
func splitNamespace(ns string) []pathSegment {
	var segments []pathSegment
	for ns != "" {
		// Check for array index.
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

		// Find next dot or bracket.
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

// --

type StructValidator[T any] struct {
	Struct T
	SL     validator.StructLevel
}

func SValidator[T any](sl validator.StructLevel) *StructValidator[T] {
	Struct := sl.Current().Interface().(T)
	validator := StructValidator[T]{
		Struct: Struct,
		SL:     sl,
	}
	return &validator
}

func (v *StructValidator[T]) Error(fieldName string, structFieldName string, msg string) {
	v.SL.ReportError(v.Struct, fieldName, structFieldName, TagMsgInParam, msg)
}
