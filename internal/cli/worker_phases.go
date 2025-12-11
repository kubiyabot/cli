package cli

import (
	"github.com/kubiyabot/cli/internal/output"
)

// WorkerPhase represents a phase in worker startup
type WorkerPhase int

const (
	// PhaseInit represents initialization phase
	PhaseInit WorkerPhase = iota
	// PhaseValidation represents pre-flight checks
	PhaseValidation
	// PhasePythonCheck represents Python environment verification
	PhasePythonCheck
	// PhaseVenvSetup represents virtual environment setup
	PhaseVenvSetup
	// PhaseDependencies represents package installation
	PhaseDependencies
	// PhaseVerification represents worker binary verification
	PhaseVerification
	// PhaseAutoUpdate represents auto-update system initialization
	PhaseAutoUpdate
	// PhaseReady represents worker ready to start
	PhaseReady
)

// PhaseNames maps phases to their display names with emojis
var PhaseNames = map[WorkerPhase]string{
	PhaseInit:         "🚀 Initialization",
	PhaseValidation:   "🔍 Pre-flight Checks",
	PhasePythonCheck:  "🐍 Python Environment",
	PhaseVenvSetup:    "📦 Virtual Environment",
	PhaseDependencies: "⚙️  Installing Dependencies",
	PhaseVerification: "✅ Verification",
	PhaseAutoUpdate:   "🔄 Auto-Update Setup",
	PhaseReady:        "🎯 Worker Ready",
}

// WorkerStartProgress manages progress display for worker startup
type WorkerStartProgress struct {
	pm           *output.ProgressManager
	currentPhase WorkerPhase
	totalPhases  int
	phaseSpinner *output.Spinner
}

// NewWorkerStartProgress creates a new worker startup progress tracker
func NewWorkerStartProgress(pm *output.ProgressManager) *WorkerStartProgress {
	return &WorkerStartProgress{
		pm:          pm,
		totalPhases: len(PhaseNames),
	}
}

// StartPhase starts a new phase
func (wsp *WorkerStartProgress) StartPhase(phase WorkerPhase) {
	wsp.currentPhase = phase
	phaseName := PhaseNames[phase]

	// Stop previous spinner if exists
	if wsp.phaseSpinner != nil {
		wsp.phaseSpinner.Stop()
	}

	// Start new phase
	phaseIndicator := wsp.pm.Phase(phaseName)
	phaseIndicator.Start()
}

// UpdateMessage updates the message for the current phase
func (wsp *WorkerStartProgress) UpdateMessage(message string) {
	wsp.pm.Info(message)
}

// CompletePhase marks the current phase as complete
func (wsp *WorkerStartProgress) CompletePhase() {
	if wsp.phaseSpinner != nil {
		wsp.phaseSpinner.Stop()
	}

	phaseName := PhaseNames[wsp.currentPhase]
	wsp.pm.Success(phaseName + " complete")
}

// FailPhase marks the current phase as failed
func (wsp *WorkerStartProgress) FailPhase(err error) {
	if wsp.phaseSpinner != nil {
		wsp.phaseSpinner.Stop()
	}

	phaseName := PhaseNames[wsp.currentPhase]
	wsp.pm.Error(phaseName + " failed: " + err.Error())
}

// Spinner creates a spinner for long-running operations within a phase
func (wsp *WorkerStartProgress) Spinner(message string) *output.Spinner {
	wsp.phaseSpinner = wsp.pm.Spinner(message)
	return wsp.phaseSpinner
}

// Success prints a success message
func (wsp *WorkerStartProgress) Success(message string) {
	wsp.pm.Success(message)
}

// Info prints an informational message
func (wsp *WorkerStartProgress) Info(message string) {
	wsp.pm.Info(message)
}

// Warning prints a warning message
func (wsp *WorkerStartProgress) Warning(message string) {
	wsp.pm.Warning(message)
}

// PrintConfig prints configuration information with a nice header
func (wsp *WorkerStartProgress) PrintConfig(config map[string]string) {
	if wsp.pm.Mode() == output.OutputModeInteractive {
		wsp.pm.Println("")
		wsp.pm.Println("╔═══════════════════════════════════════════════════════════════════╗")
		wsp.pm.Println("║              🚀  KUBIYA AGENT WORKER                             ║")
		wsp.pm.Println("╚═══════════════════════════════════════════════════════════════════╝")
		wsp.pm.Println("")
	} else {
		wsp.pm.Println("\n=== KUBIYA AGENT WORKER ===\n")
	}

	// Print config in order
	if queueID, ok := config["Queue ID"]; ok {
		wsp.pm.Printf("  📋 Queue ID:        %s\n", queueID)
	}
	if deployType, ok := config["Deployment Type"]; ok {
		wsp.pm.Printf("  🔧 Deployment:      %s\n", deployType)
	}
	if controlPlane, ok := config["Control Plane"]; ok {
		wsp.pm.Printf("  🌐 Control Plane:   %s\n", controlPlane)
	}
	wsp.pm.Println("")
}

// PrintReady prints the final ready message
func (wsp *WorkerStartProgress) PrintReady(config map[string]string) {
	if wsp.pm.Mode() == output.OutputModeInteractive {
		wsp.pm.Println("")
		wsp.pm.Println("╔═══════════════════════════════════════════════════════════════════╗")
		wsp.pm.Println("║              ✅  WORKER READY AND RUNNING                        ║")
		wsp.pm.Println("╚═══════════════════════════════════════════════════════════════════╝")
		wsp.pm.Println("")
	} else {
		wsp.pm.Println("\n=== WORKER READY ===\n")
	}

	if queueID, ok := config["Queue ID"]; ok {
		wsp.pm.Printf("  🎯 Queue ID:        %s\n", queueID)
	}
	if controlPlane, ok := config["Control Plane"]; ok {
		wsp.pm.Printf("  🔗 Control Plane:   %s\n", controlPlane)
	}
	if pkg, ok := config["Package"]; ok {
		wsp.pm.Printf("  📦 Package:         %s\n", pkg)
	}

	wsp.pm.Println("")
	if wsp.pm.Mode() == output.OutputModeInteractive {
		wsp.pm.Println("  💡 The worker is now polling for tasks")
		wsp.pm.Println("  ⌨️  Press Ctrl+C to stop gracefully")
	} else {
		wsp.pm.Println("The worker is now polling for tasks")
		wsp.pm.Println("Press Ctrl+C to stop gracefully")
	}
	wsp.pm.Println("")
}
