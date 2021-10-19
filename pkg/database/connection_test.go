package database

import (
	"fmt"
	"testing"
)

func TestConnection(t *testing.T) {
	// FIXME: Need to mock the database.
	conn := GetConnection()

	fmt.Println("Connection: ", conn)
}
