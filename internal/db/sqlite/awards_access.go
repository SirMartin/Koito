package sqlite

import "database/sql"

// SQL exposes the underlying handle. It exists only for the awards package,
// which owns its own tables and triggers outside upstream's migrations.
func (s *Sqlite) SQL() *sql.DB { return s.db }
