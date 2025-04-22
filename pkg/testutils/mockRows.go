// Copyright Contributors to the Open Cluster Management project
package testutils

import (
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
)

// ====================================================
// Mock the Row interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/rows.go#L81
// ====================================================
// type MockRow struct {
// 	MockValue []interface{}
// 	MockError error
// }

// func (r *MockRow) Scan(dest ...interface{}) error {
// 	if r.MockError != nil {
// 		return r.MockError
// 	}
// 	*dest[0].(*int) = r.MockValue[0].(int)
// 	return nil
// }

// ====================================================
// Mock the Rows interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/rows.go#L26
// ====================================================
type MockRows struct {
	MockData        []map[string]interface{}
	Index           int
	ColumnHeaders   []string
	MockErrorOnScan error
}

func (r *MockRows) Close() {}

func (r *MockRows) Err() error { return nil }

func (r *MockRows) CommandTag() pgconn.CommandTag { return nil }

func (r *MockRows) FieldDescriptions() []pgproto3.FieldDescription { return nil }

func (r *MockRows) Next() bool {
	r.Index = r.Index + 1
	return r.Index <= len(r.MockData)
}

func (r *MockRows) Scan(dest ...interface{}) error {
	if r.MockErrorOnScan != nil {
		return r.MockErrorOnScan
	}

	if len(dest) == 2 { // uid and data
		*dest[0].(*string) = r.MockData[r.Index]["uid"].(string)
		props, _ := r.MockData[r.Index]["data"].(map[string]interface{})
		dest[1] = props
	} else {
		for i := range dest {
			switch v := dest[i].(type) {
			case *int:
				// for Test_ClusterTotals test
				*dest[0].(*int) = r.MockData[r.Index]["count"].(int)
			case *string:
				*dest[i].(*string) = r.MockData[r.Index][r.ColumnHeaders[i]].(string)
			case *map[string]interface{}:
				*dest[i].(*map[string]interface{}) = r.MockData[r.Index][r.ColumnHeaders[i]].(map[string]interface{})
			case *interface{}:
				dest[i] = r.MockData[r.Index][r.ColumnHeaders[i]]
			case nil:
				fmt.Printf("error type %T", v)
			default:
				fmt.Printf("unexpected type %T", v)

			}
		}
	}
	r.Index++

	return nil
}

func (r *MockRows) Values() ([]interface{}, error) { return nil, nil }

func (r *MockRows) RawValues() [][]byte { return nil }
