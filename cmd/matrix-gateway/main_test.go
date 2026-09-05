//go:build !official_sdk

package main

import (
	"os"
	"testing"
)

func TestCheckGuardrails(t *testing.T) {
	g := checkGuardrails()
	if g.FreeNVMeGB <= 0 {
		t.Errorf("expected positive FreeNVMeGB, got %f", g.FreeNVMeGB)
	}
	if g.Status != "PASS" && g.Status != "WARN" {
		t.Errorf("unexpected status: %s", g.Status)
	}
}

func TestGetRunningSessions(t *testing.T) {
	if _, err := os.Stat(SessionsDb); os.IsNotExist(err) {
		t.Skip("sessions.db not found, skipping live session test")
	}
	sessions, err := getRunningSessions()
	if err != nil {
		t.Fatalf("getRunningSessions failed: %v", err)
	}
	t.Logf("Discovered %d running sessions in sessions.db", len(sessions))
}

func TestDispatchFleetTasks(t *testing.T) {
	if _, err := os.Stat(SessionsDb); os.IsNotExist(err) {
		t.Skip("sessions.db not found, skipping dispatch test")
	}
	err := dispatchFleetTasks()
	if err != nil {
		t.Fatalf("dispatchFleetTasks failed: %v", err)
	}
	t.Log("Successfully dispatched fleet tasks in test")
}
