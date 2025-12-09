package httpapi

import (
	"updockly/backend/internal/auth"
	"updockly/backend/internal/containers"
	"updockly/backend/internal/domain"
)

type (
	StringList            = domain.StringList
	ContainerSnapshotList = domain.ContainerSnapshotList
	JSONMap               = domain.JSONMap
	Account               = domain.Account
	ContainerSettings     = domain.ContainerSettings
	UpdateHistory         = domain.UpdateHistory
	RunningSnapshot       = domain.RunningSnapshot
	Schedule              = domain.Schedule
	Agent                 = domain.Agent
	AgentCommand          = domain.AgentCommand
	ContainerSnapshot     = domain.ContainerSnapshot
	TokenClaims           = auth.TokenClaims
	UpdateError           = containers.UpdateError
)
