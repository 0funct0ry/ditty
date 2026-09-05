package httpapi

// Compile-time assertions that the production adapters satisfy the
// interfaces NewWSHandler is written against. ErrMaxClients.CloseCode()
// propagation through sessionHubAdapter is exercised by ws_test.go's
// TestWSHandler_MaxClientsRejects1013.
var (
	_ Client = sessionClientAdapter{}
	_ Hub    = sessionHubAdapter{}
)
