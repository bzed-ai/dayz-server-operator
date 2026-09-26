// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"strings"
	"testing"
)

func TestRenderDefaultTemplate(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	msg, err := ts.Render(Event{
		Kind:     KindHealthUnhealthy,
		Instance: "deerisle",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(msg, "deerisle") || !strings.Contains(msg, "health check failing") {
		t.Errorf("Render() = %q", msg)
	}
}

func TestRenderWithData(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	msg, err := ts.Render(Event{
		Kind:     KindModUpdateDetected,
		Instance: "hashima",
		Data:     map[string]any{"Count": 3},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(msg, "hashima") || !strings.Contains(msg, "3 mod update") {
		t.Errorf("Render() = %q", msg)
	}
}

func TestRenderUnknownKindFallsBack(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	msg, err := ts.Render(Event{Kind: Kind("custom_event"), Instance: "x", Data: map[string]any{"foo": "bar"}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(msg, "custom_event") || !strings.Contains(msg, "x") {
		t.Errorf("Render() = %q", msg)
	}
}

func TestOverrideTemplate(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	if err := ts.Override(KindHealthUnhealthy, "CUSTOM {{.Instance}}"); err != nil {
		t.Fatalf("Override: %v", err)
	}
	msg, err := ts.Render(Event{Kind: KindHealthUnhealthy, Instance: "y"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if msg != "CUSTOM y" {
		t.Errorf("Render() = %q, want CUSTOM y", msg)
	}
}

func TestOverrideInvalidTemplate(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	if err := ts.Override(KindHealthUnhealthy, "{{ .Broken "); err == nil {
		t.Fatal("expected an error for invalid template syntax")
	}
}

func TestRenderCoalescedSingleEventUsesNormalTemplate(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	msg, err := ts.RenderCoalesced(KindHealthUnhealthy, []Event{{Kind: KindHealthUnhealthy, Instance: "z"}})
	if err != nil {
		t.Fatalf("RenderCoalesced: %v", err)
	}
	if !strings.Contains(msg, "z") {
		t.Errorf("RenderCoalesced() = %q", msg)
	}
}

func TestRenderCoalescedWithTemplate(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	events := []Event{
		{Kind: KindModUpdateDetected, Instance: "a"},
		{Kind: KindModUpdateDetected, Instance: "b"},
		{Kind: KindModUpdateDetected, Instance: "a"}, // duplicate instance
	}
	msg, err := ts.RenderCoalesced(KindModUpdateDetected, events)
	if err != nil {
		t.Fatalf("RenderCoalesced: %v", err)
	}
	if !strings.Contains(msg, "3") || !strings.Contains(msg, "a, b") {
		t.Errorf("RenderCoalesced() = %q", msg)
	}
}

func TestRenderCoalescedFallsBackToJoinedLines(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	events := []Event{
		{Kind: KindDriftDetected, Instance: "a", Data: map[string]any{"Path": "p1"}},
		{Kind: KindDriftDetected, Instance: "b", Data: map[string]any{"Path": "p2"}},
	}
	msg, err := ts.RenderCoalesced(KindDriftDetected, events)
	if err != nil {
		t.Fatalf("RenderCoalesced: %v", err)
	}
	lines := strings.Split(msg, "\n")
	if len(lines) != 2 {
		t.Fatalf("RenderCoalesced() = %q, want 2 lines", msg)
	}
}

func TestRenderCoalescedEmptyEvents(t *testing.T) {
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	msg, err := ts.RenderCoalesced(KindHealthUnhealthy, nil)
	if err != nil {
		t.Fatalf("RenderCoalesced: %v", err)
	}
	if msg != "" {
		t.Errorf("RenderCoalesced(nil) = %q, want empty", msg)
	}
}
