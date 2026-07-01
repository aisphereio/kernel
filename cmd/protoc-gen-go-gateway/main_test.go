package main

import "testing"

func TestDefaults(t *testing.T) {
	if got := defaultServiceName("aisphere.skill.v1.SkillService"); got != "skill-service" {
		t.Fatalf("service=%s", got)
	}
	if got := defaultRouteID(routeSpec{ServiceGoName: "SkillService", MethodName: "GetSkill"}); got != "skill.get.skill" {
		t.Fatalf("route=%s", got)
	}
}

func TestExtractPathVars(t *testing.T) {
	vars := extractPathVars("/v1/orgs/{org_id}/skills/{id=skills/*}")
	if len(vars) != 2 || vars[0] != "org_id" || vars[1] != "id=skills/*" {
		t.Fatalf("vars=%v", vars)
	}
}
