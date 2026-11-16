package pantry

import "database/sql"

// collectRows releases the connection before callers run enrichment queries.
// Querying again while holding rows can deadlock a bounded connection pool.
func collectRows[T any](rows *sql.Rows, scan func(interface{ Scan(...any) error }) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, rows.Close()
}
