package bangval_test

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/smdmitry/bang"
	"github.com/smdmitry/bang/bangval"
)

// ═══════════════════════════════════════════════════════════════════════════
//  1. БАЗОВАЯ ВАЛИДАЦИЯ STRUCT ТЕГАМИ
// ═══════════════════════════════════════════════════════════════════════════

type CreateUserRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=64"`
	Name     string `json:"name"     validate:"required,min=2,max=100"`
	Age      int    `json:"age"      validate:"omitempty,gte=18,lte=120"`
	Role     string `json:"role"     validate:"required,oneof=user admin moderator"`
}

func Example_basicStructValidation() {
	v := bangval.New()

	req := CreateUserRequest{
		Email:    "not-an-email",
		Password: "123",
		Name:     "",
		Age:      15,
		Role:     "superadmin",
	}

	err := v.Struct("users.register", req)
	if err != nil {
		// err содержит:
		// [validation] users.register: The request contains invalid data.
		//
		// Поля:
		//   key: "email",    rule: "email",    reason: "Введите корректный email-адрес"
		//   key: "password", rule: "min",      reason: "Минимальная длина — 8 символов"
		//   key: "name",     rule: "required", reason: "Поле обязательно для заполнения"
		//   key: "age",      rule: "gte",      reason: "Значение должно быть не менее 18"
		//   key: "role",     rule: "oneof",    reason: "Допустимые значения: user admin moderator"
		fmt.Println(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  2. ВЛОЖЕННЫЕ СТРУКТУРЫ И МАССИВЫ
// ═══════════════════════════════════════════════════════════════════════════

type Passport struct {
	Serial    string `json:"serial"    validate:"required,len=4,numeric"`
	Number    string `json:"number"    validate:"required,len=6,numeric"`
	IssueDate string `json:"issueDate" validate:"required"`
}

type Address struct {
	City   string `json:"city"   validate:"required"`
	Street string `json:"street" validate:"required"`
	ZIP    string `json:"zip"    validate:"required,len=6,numeric"`
}

type RegistrationRequest struct {
	Name      string     `json:"name"      validate:"required"`
	Passports []Passport `json:"passports" validate:"required,min=1,dive"`
	Address   Address    `json:"address"   validate:"required"`
}

func Example_nestedValidation() {
	v := bangval.New()

	req := RegistrationRequest{
		Name: "Иван",
		Passports: []Passport{
			{Serial: "12", Number: "123456", IssueDate: "2020-01-01"},
			{Serial: "1234", Number: "", IssueDate: ""},
		},
		Address: Address{City: "Москва", Street: "", ZIP: "abc"},
	}

	err := v.Struct("registration.create", req)
	if err != nil {
		// Поля (пути автоматически включают индексы массива):
		//   key: "passports.0.serial",    rule: "len",      reason: "Длина должна быть 4 символов"
		//   key: "passports.1.number",    rule: "required", reason: "Поле обязательно для заполнения"
		//   key: "passports.1.issueDate", rule: "required", reason: "Поле обязательно для заполнения"
		//   key: "address.street",        rule: "required", reason: "Поле обязательно для заполнения"
		//   key: "address.zip",           rule: "numeric",  reason: "Значение должно быть числом"
		fmt.Println(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  3. КАСТОМНЫЕ СООБЩЕНИЯ ДЛЯ КОНКРЕТНЫХ ПОЛЕЙ
// ═══════════════════════════════════════════════════════════════════════════

func Example_customFieldMessages() {
	v := bangval.New(bangval.WithFieldMessages(map[string]string{
		"CreateUserRequest.Email.required":    "Мы не сможем отправить подтверждение без email",
		"CreateUserRequest.Email.email":       "Проверьте email — формат должен быть user@domain.com",
		"CreateUserRequest.Password.min":      "Пароль слишком короткий, нужно минимум 8 символов",
		"CreateUserRequest.Password.required": "Придумайте пароль для входа",
	}))

	req := CreateUserRequest{Email: "bad", Password: ""}
	err := v.Struct("users.register", req)
	if err != nil {
		// key: "email",    reason: "Проверьте email — формат должен быть user@domain.com"
		// key: "password", reason: "Придумайте пароль для входа"
		fmt.Println(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  4. КАСТОМНЫЕ ВАЛИДАТОРЫ С РАЗНЫМИ RULE И REASON
// ═══════════════════════════════════════════════════════════════════════════

// ─── Определяем переиспользуемые проверки ───────────────────────────────

var dateRegex = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)

var checkPassportSerial = bangval.NewCheckFn(func(v string) *bangval.FieldError {
	if v == "" {
		return bangval.Fail("serial_required", "Укажите серию паспорта")
	}
	if len(v) != 4 {
		return bangval.Fail("serial_length", "Серия паспорта должна содержать 4 цифры")
	}
	for _, r := range v {
		if !unicode.IsDigit(r) {
			return bangval.Fail("serial_digits", "Серия паспорта должна состоять только из арабских цифр")
		}
	}
	return nil
})

var checkPassportDate = bangval.NewCheckFn(func(v string) *bangval.FieldError {
	if v == "" {
		return bangval.Fail("date_required", "Укажите дату выдачи паспорта")
	}
	if !dateRegex.MatchString(v) {
		return bangval.Fail("date_format",
			"Исправьте дату выдачи. В качестве разделителя между днём, месяцем и годом "+
				"используйте точку. Дату задавайте арабскими цифрами. Пример: 20.01.2020")
	}
	// Проверяем что дата реальная
	_, err := time.Parse("02.01.2006", v)
	if err != nil {
		return bangval.Fail("date_invalid", "Указана несуществующая дата")
	}
	return nil
})

var checkPhone = bangval.NewCheckFn(func(v string) *bangval.FieldError {
	if v == "" {
		return nil // не обязательное
	}
	if !strings.HasPrefix(v, "+") {
		return bangval.Fail("phone_plus", "Номер телефона должен начинаться с +")
	}
	digits := 0
	for _, r := range v[1:] {
		if unicode.IsDigit(r) {
			digits++
		} else if r != ' ' && r != '-' && r != '(' && r != ')' {
			return bangval.Fail("phone_chars", "Номер телефона содержит недопустимые символы")
		}
	}
	if digits < 10 || digits > 15 {
		return bangval.Fail("phone_length", "Номер телефона должен содержать от 10 до 15 цифр")
	}
	return nil
})

func Example_customValidators() {
	passports := []struct {
		Serial    string
		Number    string
		IssueDate string
	}{
		{Serial: "12AB", Number: "123456", IssueDate: "2020/01/01"},
		{Serial: "1234", Number: "654321", IssueDate: "32.13.2020"},
	}

	c := bangval.NewChecker("registration.passports")
	for i, p := range passports {
		prefix := fmt.Sprintf("passports.%d", i)
		c.Check(prefix+".serial", checkPassportSerial.Bind(p.Serial))
		c.Check(prefix+".issueDate", checkPassportDate.Bind(p.IssueDate))
	}

	err := c.Err()
	if err != nil {
		// passports.0.serial    → serial_digits  / "Серия паспорта должна состоять только из арабских цифр"
		// passports.0.issueDate → date_format    / "Исправьте дату выдачи. В качестве разделителя..."
		// passports.1.issueDate → date_invalid   / "Указана несуществующая дата"
		for _, f := range err.GetValidationFields() {
			fmt.Printf("  %s [%s]: %s\n", f.Key, f.Rule, f.Reason)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  5. ИНЛАЙН ПРОВЕРКИ (без NewCheckFn, для одноразовых проверок)
// ═══════════════════════════════════════════════════════════════════════════

func Example_inlineChecks() {
	startDate := "2025-01-15"
	endDate := "2024-12-31"

	c := bangval.NewChecker("reports.create")

	// Инлайн проверка — замыкание захватывает переменные
	c.Check("endDate", func() *bangval.FieldError {
		if endDate < startDate {
			return bangval.Fail("date_range", "Дата окончания не может быть раньше даты начала")
		}
		return nil
	})

	c.Check("startDate", func() *bangval.FieldError {
		if startDate == "" && endDate != "" {
			return bangval.Fail("start_required", "Укажите дату начала, если указана дата окончания")
		}
		return nil
	})

	err := c.Err()
	_ = err
}

// ═══════════════════════════════════════════════════════════════════════════
//  6. КОМБИНИРОВАНИЕ: struct теги + кастомные проверки
// ═══════════════════════════════════════════════════════════════════════════

type UpdateProfileRequest struct {
	Name  string `json:"name"  validate:"required,min=2"`
	Phone string `json:"phone" validate:"omitempty"` // базовая валидация тегами
	Bio   string `json:"bio"   validate:"max=500"`
}

func Example_combined() {
	v := bangval.New()

	req := UpdateProfileRequest{
		Name:  "A",
		Phone: "123-not-a-phone",
		Bio:   "Всё ок",
	}

	// 1. Struct теги → русские сообщения
	structErr := v.Struct("profile.update", req)

	// 2. Кастомные проверки
	c := bangval.NewChecker("profile.update")
	c.Check("phone", checkPhone.Bind(req.Phone))

	customErr := c.Err()

	// 3. Комбинируем
	err := bang.CombineValidation(structErr, customErr)
	if err != nil {
		// name  → min       / "Минимальная длина — 2 символов"
		// phone → phone_plus / "Номер телефона должен начинаться с +"
		fmt.Println(err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  7. ВАЛИДАЦИЯ В ХЭНДЛЕРЕ — полный пример
// ═══════════════════════════════════════════════════════════════════════════

/*
func handleCreateRegistration(w http.ResponseWriter, r *http.Request) {
    var req RegistrationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        bang.WriteHTTP(w, bang.SerializationError(err, "registration.decode"), opts)
        return
    }

    // Struct-теги
    structErr := validator.Struct("registration.create", req)

    // Кастомные проверки паспортов
    customErr := validatePassports(req.Passports)

    if combined := bang.CombineValidation(structErr, customErr); combined != nil {
        // → HTTP 422
        // {
        //   "type": "urn:error:validation:registration.create",
        //   "status": 422,
        //   "title": "Validation Error",
        //   "errorType": "validation",
        //   "errors": [
        //     {"key": "name",                   "reason": "Поле обязательно для заполнения"},
        //     {"key": "passports.0.serial",     "reason": "Серия паспорта — 4 цифры"},
        //     {"key": "passports.0.issueDate",  "reason": "Исправьте дату выдачи..."},
        //     {"key": "address.zip",            "reason": "Значение должно быть числом"}
        //   ]
        // }
        bang.WriteHTTP(w, combined, bang.HTTPResponseOptions{Instance: r.URL.Path})
        return
    }

    // OK...
}

func validatePassports(passports []Passport) *bang.Error {
    c := bangval.NewChecker("registration.passports")
    for i, p := range passports {
        prefix := fmt.Sprintf("passports.%d", i)
        c.Check(prefix+".serial", checkPassportSerial.Bind(p.Serial))
        c.Check(prefix+".issueDate", checkPassportDate.Bind(p.IssueDate))
    }
    return c.Err()
}
*/

// ═══════════════════════════════════════════════════════════════════════════
//  8. ПЕРЕОПРЕДЕЛЕНИЕ СТАНДАРТНЫХ СООБЩЕНИЙ
// ═══════════════════════════════════════════════════════════════════════════

func Example_overrideDefaultMessages() {
	v := bangval.New(bangval.WithMessages(map[string]bangval.MessageFunc{
		"required": func(fe bangval.FieldInfo) string {
			return "Это поле нужно заполнить"
		},
		"email": func(fe bangval.FieldInfo) string {
			return "Формат email: имя@домен.зона"
		},
	}))
	_ = v
}

// ═══════════════════════════════════════════════════════════════════════════
//  9. CheckAll — несколько ошибок для одного поля
// ═══════════════════════════════════════════════════════════════════════════

func Example_checkAll() {
	password := "abc"

	c := bangval.NewChecker("users.password")
	c.CheckAll("password", func() []bangval.FieldError {
		var errs []bangval.FieldError
		if len(password) < 8 {
			errs = append(errs, *bangval.Fail("password_length", "Пароль должен быть не менее 8 символов"))
		}
		hasUpper := false
		hasDigit := false
		for _, r := range password {
			if unicode.IsUpper(r) {
				hasUpper = true
			}
			if unicode.IsDigit(r) {
				hasDigit = true
			}
		}
		if !hasUpper {
			errs = append(errs, *bangval.Fail("password_upper", "Пароль должен содержать хотя бы одну заглавную букву"))
		}
		if !hasDigit {
			errs = append(errs, *bangval.Fail("password_digit", "Пароль должен содержать хотя бы одну цифру"))
		}
		return errs
	})

	err := c.Err()
	if err != nil {
		// password [password_length]: Пароль должен быть не менее 8 символов
		// password [password_upper]:  Пароль должен содержать хотя бы одну заглавную букву
		// password [password_digit]:  Пароль должен содержать хотя бы одну цифру
		for _, f := range err.GetValidationFields() {
			fmt.Printf("  %s [%s]: %s\n", f.Key, f.Rule, f.Reason)
		}
	}
}
