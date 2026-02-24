package bangvalidator

import (
	"fmt"
	"reflect"
)

// FieldInfo contains information about a failed field, passed to MessageFunc.
type FieldInfo struct {
	Field string       // field name (json tag or Go name)
	Tag   string       // validation tag that failed
	Param string       // tag parameter (e.g., "8" for min=8)
	Value any          // actual field value
	Kind  reflect.Kind // field kind (String, Int, Slice, etc.)
	Type  reflect.Type // field type
}

// MessageFunc generates a human-readable error message for a validation tag.
type MessageFunc func(fe FieldInfo) string

// defaultMessagesRu returns the default Russian message map.
func defaultMessagesRu() map[string]MessageFunc {
	return map[string]MessageFunc{
		// ─── Обязательность ─────────────────────────────────────────
		"required":             constMsg("Поле обязательно для заполнения"),
		"required_if":          constMsg("Поле обязательно для заполнения"),
		"required_unless":      constMsg("Поле обязательно для заполнения"),
		"required_with":        constMsg("Поле обязательно для заполнения"),
		"required_with_all":    constMsg("Поле обязательно для заполнения"),
		"required_without":     constMsg("Поле обязательно для заполнения"),
		"required_without_all": constMsg("Поле обязательно для заполнения"),

		// ─── Длина / размер / значение ──────────────────────────────
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

		// ─── Сравнение полей ────────────────────────────────────────
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

		// ─── Форматы ────────────────────────────────────────────────
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

		// ─── Строковые ──────────────────────────────────────────────
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

		// ─── Перечисление ───────────────────────────────────────────
		"oneof":            func(fe FieldInfo) string { return fmt.Sprintf("Допустимые значения: %s", fe.Param) },
		"excluded_with":    constMsg("Поле должно быть пустым"),
		"excluded_without": constMsg("Поле должно быть пустым"),

		// ─── Коллекции ──────────────────────────────────────────────
		"unique": constMsg("Значения не должны повторяться"),
		"dive":   constMsg("Один или несколько элементов некорректны"),

		// ─── Дата / время ───────────────────────────────────────────
		"datetime": func(fe FieldInfo) string {
			if fe.Param != "" {
				return fmt.Sprintf("Некорректный формат даты (ожидается: %s)", fe.Param)
			}
			return "Некорректный формат даты"
		},

		// ─── Файлы ──────────────────────────────────────────────────
		"file":     constMsg("Укажите корректный путь к файлу"),
		"filepath": constMsg("Укажите корректный путь"),
		"image":    constMsg("Файл должен быть изображением"),

		// ─── Банковские / финансовые ────────────────────────────────
		"credit_card": constMsg("Введите корректный номер карты"),

		// ─── Регулярные выражения ───────────────────────────────────
		"regex": constMsg("Значение не соответствует допустимому формату"),
	}
}

func constMsg(msg string) MessageFunc {
	return func(FieldInfo) string { return msg }
}
