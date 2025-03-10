package accesscontrol

import (
	"context"
	"strings"

	"github.com/grafana/grafana/pkg/models"
)

type Options struct {
	ReloadCache bool
}

type AccessControl interface {
	// Evaluate evaluates access to the given resources.
	Evaluate(ctx context.Context, user *models.SignedInUser, evaluator Evaluator) (bool, error)

	// GetUserPermissions returns user permissions.
	GetUserPermissions(ctx context.Context, user *models.SignedInUser, options Options) ([]*Permission, error)

	// GetUserRoles returns user roles.
	GetUserRoles(ctx context.Context, user *models.SignedInUser) ([]*RoleDTO, error)

	//IsDisabled returns if access control is enabled or not
	IsDisabled() bool

	// DeclareFixedRoles allows the caller to declare, to the service, fixed roles and their
	// assignments to organization roles ("Viewer", "Editor", "Admin") or "Grafana Admin"
	DeclareFixedRoles(...RoleRegistration) error

	// RegisterAttributeScopeResolver allows the caller to register a scope resolver for a
	// specific scope prefix (ex: datasources:name:)
	RegisterAttributeScopeResolver(scopePrefix string, resolver AttributeScopeResolveFunc)
}

type PermissionsProvider interface {
	GetUserPermissions(ctx context.Context, query GetUserPermissionsQuery) ([]*Permission, error)
}

type PermissionsServices interface {
	GetTeamService() PermissionsService
	GetDataSourceService() PermissionsService
}

type PermissionsService interface {
	// GetPermissions returns all permissions for given resourceID
	GetPermissions(ctx context.Context, orgID int64, resourceID string) ([]ResourcePermission, error)
	// SetUserPermission sets permission on resource for a user
	SetUserPermission(ctx context.Context, orgID int64, user User, resourceID, permission string) (*ResourcePermission, error)
	// SetTeamPermission sets permission on resource for a team
	SetTeamPermission(ctx context.Context, orgID, teamID int64, resourceID, permission string) (*ResourcePermission, error)
	// SetBuiltInRolePermission sets permission on resource for a built-in role (Admin, Editor, Viewer)
	SetBuiltInRolePermission(ctx context.Context, orgID int64, builtInRole string, resourceID string, permission string) (*ResourcePermission, error)
	// SetPermissions sets several permissions on resource for either built-in role, team or user
	SetPermissions(ctx context.Context, orgID int64, resourceID string, commands ...SetResourcePermissionCommand) ([]ResourcePermission, error)
}

type User struct {
	ID         int64
	IsExternal bool
}

// Metadata contains user accesses for a given resource
// Ex: map[string]bool{"create":true, "delete": true}
type Metadata map[string]bool

// HasGlobalAccess checks user access with globally assigned permissions only
func HasGlobalAccess(ac AccessControl, c *models.ReqContext) func(fallback func(*models.ReqContext) bool, evaluator Evaluator) bool {
	return func(fallback func(*models.ReqContext) bool, evaluator Evaluator) bool {
		if ac.IsDisabled() {
			return fallback(c)
		}

		userCopy := *c.SignedInUser
		userCopy.OrgId = GlobalOrgID
		userCopy.OrgRole = ""
		userCopy.OrgName = ""
		hasAccess, err := ac.Evaluate(c.Req.Context(), &userCopy, evaluator)
		if err != nil {
			c.Logger.Error("Error from access control system", "error", err)
			return false
		}

		return hasAccess
	}
}

func HasAccess(ac AccessControl, c *models.ReqContext) func(fallback func(*models.ReqContext) bool, evaluator Evaluator) bool {
	return func(fallback func(*models.ReqContext) bool, evaluator Evaluator) bool {
		if ac.IsDisabled() {
			return fallback(c)
		}

		hasAccess, err := ac.Evaluate(c.Req.Context(), c.SignedInUser, evaluator)
		if err != nil {
			c.Logger.Error("Error from access control system", "error", err)
			return false
		}

		return hasAccess
	}
}

var ReqGrafanaAdmin = func(c *models.ReqContext) bool {
	return c.IsGrafanaAdmin
}

var ReqOrgAdmin = func(c *models.ReqContext) bool {
	return c.OrgRole == models.ROLE_ADMIN
}

func BuildPermissionsMap(permissions []*Permission) map[string]bool {
	permissionsMap := make(map[string]bool)
	for _, p := range permissions {
		permissionsMap[p.Action] = true
	}

	return permissionsMap
}

// GroupScopesByAction will group scopes on action
func GroupScopesByAction(permissions []*Permission) map[string][]string {
	m := make(map[string][]string)
	for _, p := range permissions {
		m[p.Action] = append(m[p.Action], p.Scope)
	}
	return m
}

func ValidateScope(scope string) bool {
	prefix, last := scope[:len(scope)-1], scope[len(scope)-1]
	// verify that last char is either ':' or '/' if last character of scope is '*'
	if len(prefix) > 0 && last == '*' {
		lastChar := prefix[len(prefix)-1]
		if lastChar != ':' && lastChar != '/' {
			return false
		}
	}
	return !strings.ContainsAny(prefix, "*?")
}

func addActionToMetadata(allMetadata map[string]Metadata, action, id string) map[string]Metadata {
	metadata, initialized := allMetadata[id]
	if !initialized {
		metadata = Metadata{action: true}
	} else {
		metadata[action] = true
	}
	allMetadata[id] = metadata
	return allMetadata
}

// GetResourcesMetadata returns a map of accesscontrol metadata, listing for each resource, users available actions
func GetResourcesMetadata(ctx context.Context, permissions []*Permission, resource string, resourceIDs map[string]bool) map[string]Metadata {
	allScope := GetResourceAllScope(resource)
	allIDScope := GetResourceAllIDScope(resource)

	// prefix of ID based scopes (resource:id)
	idPrefix := Scope(resource, "id")
	// index of the ID in the scope
	idIndex := len(idPrefix) + 1

	// Loop through permissions once
	result := map[string]Metadata{}
	for _, p := range permissions {
		if p.Scope == "*" || p.Scope == allScope || p.Scope == allIDScope {
			// Add global action to all resources
			for id := range resourceIDs {
				result = addActionToMetadata(result, p.Action, id)
			}
		} else {
			if len(p.Scope) > idIndex && strings.HasPrefix(p.Scope, idPrefix) && resourceIDs[p.Scope[idIndex:]] {
				// Add action to a specific resource
				result = addActionToMetadata(result, p.Action, p.Scope[idIndex:])
			}
		}
	}

	return result
}
