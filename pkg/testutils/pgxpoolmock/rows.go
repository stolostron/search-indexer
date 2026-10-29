// Copyright Contributors to the Open Cluster Management project

package pgxpoolmock

import (
	pgx "github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

// Rows is a light wrapper used by existing tests.
type Rows struct {
	rows *pgxmock.Rows
}

func NewRows(columns []string) *Rows {
	return &Rows{rows: pgxmock.NewRows(columns)}
}

func (r *Rows) AddRow(values ...interface{}) *Rows {
	vals := make([]any, len(values))
	for i, v := range values {
		vals[i] = v
	}
	r.rows.AddRow(vals...)
	return r
}

func (r *Rows) ToPgxRows() pgx.Rows {
	return r.rows.Kind()
}
