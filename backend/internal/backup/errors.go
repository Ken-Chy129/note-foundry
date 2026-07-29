package backup

import "errors"

var ErrNoBackup = errors.New("no encrypted backup is available")
