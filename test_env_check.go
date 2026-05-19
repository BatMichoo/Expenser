package main

import (
	"fmt"
	"os"
)

func main() {
	vars := []string{"DB_USER", "DB_PASS", "DB_HOST", "DB_PORT", "DB_NAME", "TEST_DB_NAME"}
	for _, v := range vars {
		fmt.Printf("%s: %s\n", v, os.Getenv(v))
	}
}
