// Package bang provides structured, classified errors with fluent construction,
// registration, RFC 7807 HTTP responses, and structured logging.
//
// # Quick Start
//
//	// Direct usage — minimal calls
//	bang.NotFound().Code("users.get_by_id")
//	bang.Database().Wrap(err).Code("users.create").Debug("email", email)
//
//	// Fluent builder
//	bang.NotFound().Wrap(err).
//	    Safe("key", "value").
//	    Debug("query", q).
//	    Code("users.get")
//
// # Registration
//
//	// Register messages
//	bang.MustRegisterMessages(map[string]string{
//	    "task.not_found": "Task '{{.key}}' not found",
//	})
//
//	// Register full config
//	bang.MustRegisterCodes(map[string]bang.CodeCfg{
//	    "task.not_found": {
//	        Class: bang.ClassNotFound,
//	        Msg:   "Task '{{.key}}' not found",
//	    },
//	})
//
//	// Factory
//	var TaskNotFound = bang.Factory("task.not_found")
//	TaskNotFound().Debug(...)
//
//	// Register builder
//	var TaskNotFound = bang.Register("task.not_found").
//	    Class(bang.ClassNotFound).
//	    Message("Task '{{.key}}' not found").
//	    New()
//
// # Typed data
//
//	var TaskNotFound = bang.RegisterTyped[TaskNotFoundData]("task.not_found").
//	    Class(bang.ClassNotFound).
//	    Message("Task '{{.task_id}}' not found").
//	    New()
//	TaskNotFound(TaskNotFoundData{TaskID: "123"})
//
// # Validation
//
//	bang.Validation().Code("users.register").
//	    Field("email", "email_format", "Введите корректный email.").
//	    Field("password", "min_length", "Пароль — не менее 8 символов.")
//
// # HTTP (RFC 7807)
//
//	bang.WriteHTTP(w, err, bang.HTTPResponseOptions{Instance: r.URL.Path})
//
// # Logging
//
//	slog.Error("op failed", bang.SlogAttrs(err)...)
//	bang.SlogError(logger, "op failed", err)
//
// # Inspection
//
//	bang.HasClass(err, bang.ClassNotFound)
//	bang.HasCode(err, "users.not_found")
//	errors.Is(err, bang.NotFound())
//	data, ok := bang.GetData[OrderData](err)
package bang
