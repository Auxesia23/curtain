package services

import "errors"

var (
	ErrEmailTaken          = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrUserNotFound        = errors.New("user not found")
	ErrProjectNotFound     = errors.New("project not found")
	ErrForbidden           = errors.New("forbidden: not a project member")
	ErrAlreadyMember       = errors.New("user is already a project member")
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrEnvironmentTaken    = errors.New("environment name already exists in this project")
	ErrVariableNotFound    = errors.New("variable not found")

	// Authentication and user administration.
	ErrUserDisabled   = errors.New("account is disabled")
	ErrInviteRequired = errors.New("registration requires a valid invite link")
	ErrInviteInvalid  = errors.New("invite link is invalid, expired, or fully used")
	ErrInviteNotFound = errors.New("invite not found")
	ErrSelfAction     = errors.New("cannot perform this action on your own account")
	ErrLastAdmin      = errors.New("cannot remove or disable the last active admin")
	ErrNotAdmin       = errors.New("forbidden: admin only")

	// Project roles and capabilities.
	ErrNotMember            = errors.New("user is not a project member")
	ErrLastOwner            = errors.New("cannot remove or demote the last owner of the project")
	ErrInviteOwnerForbidden = errors.New("inviting an owner requires the member.manage capability")
	ErrInvalidCapability    = errors.New("unknown capability")
	ErrDuplicateCapability  = errors.New("duplicate capability in overrides")
)
