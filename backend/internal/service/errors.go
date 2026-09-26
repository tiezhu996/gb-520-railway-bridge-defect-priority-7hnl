package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrDecisionLocked    = errors.New("final priority decisions are immutable")
	ErrReviewRole        = errors.New("reviewer or admin role is required to finalize a priority")
	ErrSeparationOfDuty  = errors.New("priority preparer cannot approve the same decision")
	ErrNotDecisionOwner  = errors.New("only the preparer may edit this draft decision")
	ErrReleaseRole       = errors.New("reviewer or admin role is required to release a priority decision")
	ErrReleaseNotActive  = errors.New("only finalized restrict/urgent decisions can be released")
	ErrAlreadyReleased   = errors.New("priority decision has already been released")
	ErrDefectsPending    = errors.New("bridge defects must all be mitigated or closed before release")
	ErrNoBridgeDefects   = errors.New("no defects are recorded for this bridge; cannot confirm release clearance")
)
