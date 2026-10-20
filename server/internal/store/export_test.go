package store

import "database/sql"

func (s *Store) Reader() *sql.DB { return s.reader }

func (s *Store) SetAfterResolve(f func()) { s.afterResolve = f }
