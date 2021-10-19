// Copyright Contributors to the Open Cluster Management project

package database

import (
	"fmt"
	"testing"
)

func TestConnection(t *testing.T) {
	// TODO: Need to mock the database.
	conn := GetConnection()

	fmt.Println("TODO: This is a dummy test, need to mock database.")
	fmt.Println("Connection: ", conn)
}
