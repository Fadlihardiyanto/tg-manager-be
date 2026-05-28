package converter

// This file provides a centralized place for converter utility functions
// All converters are organized in separate files for maintainability

// Package converter contains functions to convert database entities to API models (DTOs)
//
// Usage examples:
//
//   user := &entity.User{...}
//   userResponse := converter.UserToResponse(user)
//
//   users := []entity.User{...}
//   userResponses := converter.UsersToResponse(users)
