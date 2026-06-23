// SPDX-License-Identifier: GPL-3.0-only
package platform

import "testing"

func TestNewKillSwitchNonNil(t *testing.T) {
	if NewKillSwitch() == nil {
		t.Fatal("NewKillSwitch returned nil")
	}
}

func TestRuleNameStable(t *testing.T) {
	if RuleName == "" {
		t.Fatal("RuleName must be set for cleanup")
	}
}
