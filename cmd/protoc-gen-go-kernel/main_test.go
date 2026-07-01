package main

import "testing"

func TestDefaultServiceName(t *testing.T) {
	if got := defaultServiceName("aisphere.skill.v1.SkillService"); got != "skill-service" {
		t.Fatalf("service=%s", got)
	}
}
