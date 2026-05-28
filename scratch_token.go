package main

import (
	"fmt"
	"log"

	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
)

func main() {
	tokenStr := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMmI4NTcxNWUtNGE3Yi00N2UwLWFlMjktMTY4YmE4NWJkNTQ5IiwiY2xpZW50X2lkIjoiZmRjYTIyMWYtNmIyMi00NjcwLTljM2UtNDk2YmI3ZjRiNDJlIiwicm9sZSI6Im93bmVyIiwicGVybWlzc2lvbnMiOltdLCJ0eXBlIjoidGVuYW50IiwiaXNzIjoidGctbWFuYWdlciIsInN1YiI6IjJiODU3MTVlLTRhN2ItNDdlMC1hZTI5LTE2OGJhODViZDU0OSIsImF1ZCI6WyJ0ZW5hbnQiXSwiZXhwIjoxNzc5NjM2MTg3LCJpYXQiOjE3Nzk2MzUyODcsImp0aSI6ImE5MTU0ZDFiLWM3NWItNDc4NC1iZDk4LTUxNTlkMTBjZmQ0ZiJ9.ew9KLsSvZYBwkcf9iQ9ouKPL7vJA5gX4_9bcyi6UjyY"
	secretKey := ""

	claims, err := pkg_jwt.ParseTenantToken(tokenStr, secretKey)
	if err != nil {
		log.Fatalf("Error parsing token: %v\n", err)
	}

	fmt.Printf("Parsed successfully: %+v\n", claims)
}
