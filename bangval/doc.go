// Package bangval integrates go-playground/validator/v10 with bang,
// providing Russian human-readable messages and custom multi-condition validators.
//
// # Standard validation via struct tags
//
//	v := bangval.New()
//	err := v.Struct("users.register", req)
//	// → *bang.Error{ClassValidation} with Russian messages, or nil
//
// # Custom validators with multiple conditions and different messages
//
//	c := bangval.NewChecker("users.register")
//	c.Check("passports.0.serial", func() *bangval.FieldError {
//	    if len(serial) != 4 {
//	        return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
//	    }
//	    if !allDigits(serial) {
//	        return bangval.Fail("serial_digits", "Серия должна состоять из цифр")
//	    }
//	    return nil
//	})
//	err := c.Err()
//
// # Combine struct validation + custom checks
//
//	err := bang.CombineValidation(
//	    v.Struct("users.register", req),
//	    validatePassports(req.Passports),
//	)
//
// # Custom messages for specific fields
//
//	v := bangval.New(bangval.WithFieldMessages(map[string]string{
//	    "User.Email.required":  "Пожалуйста, укажите email",
//	    "User.Name.min":        "Имя слишком короткое",
//	}))
//
// # Override default Russian messages
//
//	v := bangval.New(bangval.WithMessages(map[string]bangval.MessageFunc{
//	    "required": func(fe FieldInfo) string { return "Это поле нужно заполнить" },
//	}))
package bangval
