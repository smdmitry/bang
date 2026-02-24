// Package bang provides structured, classified errors with flexible construction patterns.
//
// # Design Philosophy
//
// bang is built around the idea that errors should be easy to create, carry rich metadata,
// and support both human-readable output (HTTP responses) and machine-readable output (logs, metrics).
//
// Every error has a Class (category), a Code (unique identifier), and standard fields for
// Title, Message, Data, and Debug information. File and line are captured automatically.
//
// # Quick Start
//
// Direct usage with shorthand constructors:
//
//	return bang.NotFound("users.get_by_id")
//	return bang.NotFound(err, "users.get_by_id")
//	return bang.DatabaseError(err).DebugKV("id", id)
//
// Module prefix for a package:
//
//	var Mod = bang.NewModule("orders.repo")
//	return Mod.NotFound(err, "get_by_id")  // code → "orders.repo.get_by_id"
//
// Pre-defined callable error factories:
//
//	var ErrUserNotFound = bang.Define(bang.ClassNotFound, "users.not_found")
//
//	return ErrUserNotFound()
//	return ErrUserNotFound(err)
//	return ErrUserNotFound(err).Msgf("user %s not found", id)
//
// Typed data factories:
//
//	var ErrOrderFailed = bang.DefineTyped[OrderData](bang.ClassDatabaseError, "orders.save_failed")
//
//	return ErrOrderFailed().Data(OrderData{OrderID: id})
//	return ErrOrderFailed(err).Data(OrderData{OrderID: id})
//
// Message templates with placeholders:
//
//	var ErrProductMissing = bang.Define(bang.ClassNotFound, "products.missing",
//	    bang.WithMsgTpl("товар {sku} не найден на складе {warehouse}"),
//	)
//	return ErrProductMissing().DebugKV("sku", "X1", "warehouse", "msk-1")
//	// message → "товар X1 не найден на складе msk-1"
//
// Validation errors:
//
//	return bang.Validation("users.register").
//	    Field("email", "email_format", "Please enter a valid email.").
//	    Field("password", "min_length", "Password must be at least 8 characters.")
//
// HTTP response (RFC 7807):
//
//	bang.WriteHTTP(w, err, bang.HTTPResponseOptions{Instance: r.URL.Path})
//
// Logging:
//
//	slog.Error("operation failed", bang.SlogAttrs(err)...)
//
// Error inspection:
//
//	bang.HasClass(err, bang.ClassNotFound)
//	bang.HasCode(err, "users.not_found")
//	ErrUserNotFound.Is(err)
//	data, ok := bang.GetData[OrderData](err)
package bang
