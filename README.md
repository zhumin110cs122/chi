# Chi Timeout Fix

## Fix: middleware.Timeout now properly cancels request context after timeout

### Problem
The `middleware.Timeout` handler did not properly cancel the request context when the timeout
expired. This allowed handler goroutines to continue running indefinitely, potentially
causing goroutine leaks and resource exhaustion.

### Fix
The Timeout middleware now correctly cancels the request context after the specified
duration, ensuring that handlers receive the cancellation signal via `r.Context().Done()`
and can clean up resources promptly.

### Test
```bash
go run main.go
```
