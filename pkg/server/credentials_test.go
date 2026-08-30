package server

import (
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
)

func TestInstanceCredentialsCarryEveryConfiguredSecret(t *testing.T) {
	inst := &config.Instance{APIKey: "key", Username: "user", Password: "pw"}

	got := InstanceCredentials(inst)
	if got.APIKey != "key" || got.Username != "user" || got.Password != "pw" {
		t.Errorf("InstanceCredentials = %+v, want key/user/pw", got)
	}
}
