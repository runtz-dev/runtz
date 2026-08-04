package api

import (
	"testing"

	"github.com/runtz-dev/runtz/engine/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestGlobalDataScopeIsSelfHostedAdminOnly(t *testing.T) {
	admin := User{Role: "admin"}
	member := User{Role: "member"}

	selfHosted := &Server{cfg: config.Config{DeploymentMode: hostingSelfHosted}}
	if !selfHosted.globalDataScope(admin) {
		t.Fatal("self-hosted admin should read across every workspace")
	}
	if selfHosted.globalDataScope(member) {
		t.Fatal("self-hosted member should be limited to their workspaces")
	}

	// In cloud each workspace is a different customer, so no role widens data
	// access: an admin row appearing in the database must not expose tenants
	// to each other.
	cloud := &Server{cfg: config.Config{DeploymentMode: hostingCloud}}
	if cloud.globalDataScope(admin) {
		t.Fatal("cloud admin must not read across tenants")
	}
	if cloud.globalDataScope(member) {
		t.Fatal("cloud member must not read across tenants")
	}
}

func TestUserCanAccessWorkspaceHonoursDeploymentMode(t *testing.T) {
	own := bson.NewObjectID()
	other := bson.NewObjectID()
	admin := User{Role: "admin", WorkspaceIDs: []bson.ObjectID{own}}

	selfHosted := &Server{cfg: config.Config{DeploymentMode: hostingSelfHosted}}
	if !selfHosted.userCanAccessWorkspace(admin, other) {
		t.Fatal("self-hosted admin should reach a workspace they do not belong to")
	}

	cloud := &Server{cfg: config.Config{DeploymentMode: hostingCloud}}
	if cloud.userCanAccessWorkspace(admin, other) {
		t.Fatal("cloud admin reached another tenant's workspace")
	}
	if !cloud.userCanAccessWorkspace(admin, own) {
		t.Fatal("cloud admin lost access to their own workspace")
	}
}
