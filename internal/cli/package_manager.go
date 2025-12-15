package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// PackageManager interface abstracts package installation operations
type PackageManager interface {
	Name() string
	Install(ctx context.Context, venvPath, packageSpec string, quiet, forceReinstall bool) error
	Show(venvPath, packageName string) (string, error) // returns version
	ClearCache() error
}

// NewPackageManager returns appropriate package manager (UV or pip)
func NewPackageManager() PackageManager {
	// Check if uv is available
	if _, err := exec.LookPath("uv"); err == nil {
		return &UVPackageManager{}
	}
	return &PipPackageManager{}
}

// UVPackageManager uses uv for package management
type UVPackageManager struct{}

func (u *UVPackageManager) Name() string {
	return "uv"
}

func (u *UVPackageManager) Install(ctx context.Context, venvPath, packageSpec string, quiet, forceReinstall bool) error {
	// uv pip install command
	args := []string{"pip", "install"}

	if quiet {
		args = append(args, "--quiet")
	}

	if forceReinstall {
		args = append(args, "--force-reinstall")
		// Note: Don't use --no-deps as it prevents extras like [worker] from installing dependencies
	}

	args = append(args, packageSpec)

	// Set environment to use the specific venv
	cmd := exec.CommandContext(ctx, "uv", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("VIRTUAL_ENV=%s", venvPath))

	// Capture output for better error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("uv install failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (u *UVPackageManager) Show(venvPath, packageName string) (string, error) {
	cmd := exec.Command("uv", "pip", "show", packageName)
	cmd.Env = append(os.Environ(), fmt.Sprintf("VIRTUAL_ENV=%s", venvPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			return version, nil
		}
	}

	return "", fmt.Errorf("version not found in output")
}

func (u *UVPackageManager) ClearCache() error {
	cmd := exec.Command("uv", "cache", "clean")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clear uv cache: %w", err)
	}
	return nil
}

// PipPackageManager uses pip for package management (fallback)
type PipPackageManager struct{}

func (p *PipPackageManager) Name() string {
	return "pip"
}

func (p *PipPackageManager) Install(ctx context.Context, venvPath, packageSpec string, quiet, forceReinstall bool) error {
	pipPath := getPipPath(venvPath)

	args := []string{"install"}

	if quiet {
		args = append(args, "--quiet")
	}

	if forceReinstall {
		args = append(args, "--force-reinstall")
	}

	args = append(args, packageSpec)

	cmd := exec.CommandContext(ctx, pipPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pip install failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (p *PipPackageManager) Show(venvPath, packageName string) (string, error) {
	pipPath := getPipPath(venvPath)

	cmd := exec.Command(pipPath, "show", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			return version, nil
		}
	}

	return "", fmt.Errorf("version not found in output")
}

func (p *PipPackageManager) ClearCache() error {
	// Try pip cache purge
	cmd := exec.Command("pip", "cache", "purge")
	if err := cmd.Run(); err != nil {
		// Try pip3 as fallback
		cmd = exec.Command("pip3", "cache", "purge")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clear pip cache: %w", err)
		}
	}
	return nil
}

// getPipPath returns the path to pip in the venv
func getPipPath(venvPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s\\Scripts\\pip.exe", venvPath)
	}
	return fmt.Sprintf("%s/bin/pip", venvPath)
}

// UpgradePip upgrades pip in the venv (only needed for PipPackageManager)
func UpgradePip(venvPath string, quiet bool) error {
	pipPath := getPipPath(venvPath)

	args := []string{"install", "--upgrade", "pip"}
	if quiet {
		args = append(args, "--quiet")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pipPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade pip: %w", err)
	}

	return nil
}
