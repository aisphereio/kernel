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

func TestRouteUpstreamService(t *testing.T) {
	tests := []struct {
		name  string
		route routeSpec
		want  string
	}{
		{
			name:  "explicit gateway target",
			route: routeSpec{ServiceFullName: "skill.v1.SkillService", Audience: "legacy-service", UpstreamService: "hub-service"},
			want:  "hub-service",
		},
		{
			name:  "legacy authz audience",
			route: routeSpec{ServiceFullName: "skill.v1.SkillService", Audience: "hub-service"},
			want:  "hub-service",
		},
		{
			name:  "service default",
			route: routeSpec{ServiceFullName: "skill.v1.SkillService"},
			want:  "skill-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routeUpstreamService(tt.route); got != tt.want {
				t.Fatalf("service=%q want=%q", got, tt.want)
			}
		})
	}
}
