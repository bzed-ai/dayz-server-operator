// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bzed-ai/dayz-server-operator/internal/quadlet"
)

func TestWriteContainerUnit(t *testing.T) {
	dir := t.TempDir()
	spec := quadlet.ContainerSpec{Name: "dzo-deerisle", Image: "localhost/dzo-runtime:latest", Network: quadlet.NetworkHost}

	path, err := WriteContainerUnit(dir, spec)
	if err != nil {
		t.Fatalf("WriteContainerUnit: %v", err)
	}
	if path != filepath.Join(dir, "dzo-deerisle.container") {
		t.Errorf("path = %q", path)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "Image=localhost/dzo-runtime:latest") {
		t.Errorf("content = %q", data)
	}
}

func TestWriteContainerUnitInvalidSpecErrors(t *testing.T) {
	if _, err := WriteContainerUnit(t.TempDir(), quadlet.ContainerSpec{}); err == nil {
		t.Fatal("expected an error for an invalid spec")
	}
}

func TestWriteContainerUnitMissingDirErrors(t *testing.T) {
	spec := quadlet.ContainerSpec{Name: "dzo-x", Image: "img", Network: quadlet.NetworkHost}
	if _, err := WriteContainerUnit(filepath.Join(t.TempDir(), "missing"), spec); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestWriteContainerUnitOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	spec1 := quadlet.ContainerSpec{Name: "dzo-x", Image: "img1", Network: quadlet.NetworkHost}
	spec2 := quadlet.ContainerSpec{Name: "dzo-x", Image: "img2", Network: quadlet.NetworkHost}

	path, err := WriteContainerUnit(dir, spec1)
	if err != nil {
		t.Fatalf("WriteContainerUnit: %v", err)
	}
	if _, err := WriteContainerUnit(dir, spec2); err != nil {
		t.Fatalf("WriteContainerUnit (overwrite): %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "img2") {
		t.Errorf("content should reflect the overwrite: %q", data)
	}
}

func TestTimerUnitRenderAndWrite(t *testing.T) {
	dir := t.TempDir()
	unit := TimerUnit{
		Name:        "dzo-restart-deerisle",
		Description: "Maintenance restart for deerisle",
		ExecStart:   "/usr/local/bin/dzo restart deerisle --graceful",
		OnCalendar:  []string{"Mon..Fri 04:00", "Sat,Sun 06:00"},
		Persistent:  false,
	}
	timerPath, servicePath, err := WriteTimerUnit(dir, unit)
	if err != nil {
		t.Fatalf("WriteTimerUnit: %v", err)
	}

	timerData, err := os.ReadFile(timerPath) //nolint:gosec // test fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read timer: %v", err)
	}
	if !strings.Contains(string(timerData), "OnCalendar=Mon..Fri 04:00") ||
		!strings.Contains(string(timerData), "OnCalendar=Sat,Sun 06:00") ||
		!strings.Contains(string(timerData), "Persistent=false") ||
		!strings.Contains(string(timerData), "WantedBy=timers.target") {
		t.Errorf("timer content = %q", timerData)
	}

	serviceData, err := os.ReadFile(servicePath) //nolint:gosec // test fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	if !strings.Contains(string(serviceData), "Type=oneshot") ||
		!strings.Contains(string(serviceData), "ExecStart=/usr/local/bin/dzo restart deerisle --graceful") {
		t.Errorf("service content = %q", serviceData)
	}
}

func TestTimerUnitValidate(t *testing.T) {
	cases := []TimerUnit{
		{},
		{Name: "x"},
		{Name: "x", ExecStart: "cmd"},
	}
	for _, u := range cases {
		if err := u.Validate(); err == nil {
			t.Errorf("Validate(%+v): expected an error", u)
		}
	}
	ok := TimerUnit{Name: "x", ExecStart: "cmd", OnCalendar: []string{"daily"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate(%+v): unexpected error %v", ok, err)
	}
}

func TestWriteTimerUnitMissingDirErrors(t *testing.T) {
	unit := TimerUnit{Name: "x", ExecStart: "cmd", OnCalendar: []string{"daily"}}
	if _, _, err := WriteTimerUnit(filepath.Join(t.TempDir(), "missing"), unit); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestTimerUnitNoDescriptionOmitsUnitSection(t *testing.T) {
	unit := TimerUnit{Name: "x", ExecStart: "cmd", OnCalendar: []string{"daily"}}
	timer, err := unit.RenderTimer()
	if err != nil {
		t.Fatalf("RenderTimer: %v", err)
	}
	if strings.Contains(timer, "[Unit]") {
		t.Errorf("timer = %q, want no [Unit] section without a Description", timer)
	}
}
