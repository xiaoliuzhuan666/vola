package sqlite

import (
	"errors"
	"strings"

	moderncsqlite "modernc.org/sqlite"
)

// IsBusyLockedError reports transient SQLite write contention errors.
func IsBusyLockedError(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *moderncsqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		if code&0xff == 5 || code&0xff == 6 {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "sqlite_busy") ||
		strings.Contains(message, "sqlite_locked")
}
