package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCoverageReport generates and validates test coverage.
func TestCoverageReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coverage test in short mode")
	}

	// Get project root
	projectRoot, err := getProjectRoot()
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Generate coverage profile
	coverageFile := filepath.Join(projectRoot, "coverage.out")
	defer os.Remove(coverageFile)

	cmd := exec.Command("go", "test", "-coverprofile="+coverageFile, "./...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Test output:\n%s", output)
		// Don't fail on test errors, just report coverage
	}

	// Parse coverage
	coverage, err := parseCoverageFile(coverageFile)
	if err != nil {
		t.Fatalf("Failed to parse coverage: %v", err)
	}

	// Print coverage report
	t.Log("\n=== Code Coverage Report ===")
	printCoverageReport(t, coverage)

	// Verify coverage targets
	verifyCoverageTargets(t, coverage)
}

// Coverage represents coverage statistics for a package.
type Coverage struct {
	Package    string
	Statements int
	Covered    int
	Percentage float64
}

// parseCoverageFile parses a coverage profile file.
func parseCoverageFile(path string) ([]Coverage, error) {
	// Use go tool cover to generate coverage report
	cmd := exec.Command("go", "tool", "cover", "-func="+path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run go tool cover: %w", err)
	}

	return parseCoverageOutput(string(output)), nil
}

// parseCoverageOutput parses the output of go tool cover.
func parseCoverageOutput(output string) []Coverage {
	var result []Coverage
	packageCoverage := make(map[string]*Coverage)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total:") {
			continue
		}

		// Parse line: path/to/file.go:123.45:  functionName  67.8%
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Extract file path and coverage percentage
		filePath := parts[0]
		coverageStr := parts[len(parts)-1]

		// Extract package name from file path
		pkg := extractPackage(filePath)
		if pkg == "" {
			continue
		}

		// Parse coverage percentage
		coverageStr = strings.TrimSuffix(coverageStr, "%")
		percentage, err := strconv.ParseFloat(coverageStr, 64)
		if err != nil {
			continue
		}

		// Update package coverage
		if packageCoverage[pkg] == nil {
			packageCoverage[pkg] = &Coverage{
				Package: pkg,
			}
		}
		packageCoverage[pkg].Statements++
		if percentage > 0 {
			packageCoverage[pkg].Covered++
		}
	}

	// Calculate percentages
	for _, cov := range packageCoverage {
		if cov.Statements > 0 {
			cov.Percentage = float64(cov.Covered) / float64(cov.Statements) * 100
		}
		result = append(result, *cov)
	}

	return result
}

// extractPackage extracts package name from file path.
func extractPackage(filePath string) string {
	// Remove line number suffix
	idx := strings.Index(filePath, ":")
	if idx > 0 {
		filePath = filePath[:idx]
	}

	// Extract directory
	dir := filepath.Dir(filePath)

	// Get last component for package name
	parts := strings.Split(dir, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}

	// Handle internal packages
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "claude-agent-sdk-go" {
			if i+1 < len(parts) {
				return strings.Join(parts[i+1:], "/")
			}
			return "root"
		}
	}

	return parts[len(parts)-1]
}

// printCoverageReport prints a formatted coverage report.
func printCoverageReport(t *testing.T, coverage []Coverage) {
	t.Helper()

	if len(coverage) == 0 {
		t.Log("No coverage data available")
		return
	}

	// Group by category
	publicPackages := []Coverage{}
	internalPackages := []Coverage{}

	for _, cov := range coverage {
		if strings.HasPrefix(cov.Package, "internal") {
			internalPackages = append(internalPackages, cov)
		} else {
			publicPackages = append(publicPackages, cov)
		}
	}

	// Print public API coverage
	if len(publicPackages) > 0 {
		t.Log("\nPublic API Coverage:")
		totalStatements := 0
		totalCovered := 0
		for _, cov := range publicPackages {
			t.Logf("  %-30s %6.2f%%", cov.Package, cov.Percentage)
			totalStatements += cov.Statements
			totalCovered += cov.Covered
		}
		if totalStatements > 0 {
			overall := float64(totalCovered) / float64(totalStatements) * 100
			t.Logf("  %-30s %6.2f%%", "PUBLIC TOTAL", overall)
		}
	}

	// Print internal package coverage
	if len(internalPackages) > 0 {
		t.Log("\nInternal Package Coverage:")
		totalStatements := 0
		totalCovered := 0
		for _, cov := range internalPackages {
			t.Logf("  %-30s %6.2f%%", cov.Package, cov.Percentage)
			totalStatements += cov.Statements
			totalCovered += cov.Covered
		}
		if totalStatements > 0 {
			overall := float64(totalCovered) / float64(totalStatements) * 100
			t.Logf("  %-30s %6.2f%%", "INTERNAL TOTAL", overall)
		}
	}

	// Print overall
	totalStatements := 0
	totalCovered := 0
	for _, cov := range coverage {
		totalStatements += cov.Statements
		totalCovered += cov.Covered
	}
	if totalStatements > 0 {
		overall := float64(totalCovered) / float64(totalStatements) * 100
		t.Logf("\n  %-30s %6.2f%%", "OVERALL COVERAGE", overall)
	}

	t.Log("\n============================")
}

// verifyCoverageTargets checks if coverage meets targets.
func verifyCoverageTargets(t *testing.T, coverage []Coverage) {
	t.Helper()

	// Coverage targets
	const (
		publicAPITarget   = 85.0
		internalAPITarget = 80.0
	)

	// Calculate coverage by category
	publicStatements, publicCovered := 0, 0
	internalStatements, internalCovered := 0, 0

	for _, cov := range coverage {
		if strings.HasPrefix(cov.Package, "internal") {
			internalStatements += cov.Statements
			internalCovered += cov.Covered
		} else {
			publicStatements += cov.Statements
			publicCovered += cov.Covered
		}
	}

	// Verify public API coverage
	if publicStatements > 0 {
		publicCoverage := float64(publicCovered) / float64(publicStatements) * 100
		if publicCoverage < publicAPITarget {
			t.Logf("WARNING: Public API coverage %.2f%% is below target %.2f%%",
				publicCoverage, publicAPITarget)
		} else {
			t.Logf("Public API coverage %.2f%% meets target %.2f%%",
				publicCoverage, publicAPITarget)
		}
	}

	// Verify internal package coverage
	if internalStatements > 0 {
		internalCoverage := float64(internalCovered) / float64(internalStatements) * 100
		if internalCoverage < internalAPITarget {
			t.Logf("WARNING: Internal package coverage %.2f%% is below target %.2f%%",
				internalCoverage, internalAPITarget)
		} else {
			t.Logf("Internal package coverage %.2f%% meets target %.2f%%",
				internalCoverage, internalAPITarget)
		}
	}
}

// getProjectRoot returns the project root directory.
func getProjectRoot() (string, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up to find go.mod
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return cwd, nil
		}
		dir = parent
	}
}

// TestCoverageHTML generates an HTML coverage report.
func TestCoverageHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTML coverage generation in short mode")
	}

	// Get project root
	projectRoot, err := getProjectRoot()
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Generate coverage profile
	coverageFile := filepath.Join(projectRoot, "coverage.out")
	htmlFile := filepath.Join(projectRoot, "coverage.html")

	// Generate coverage
	cmd := exec.Command("go", "test", "-coverprofile="+coverageFile, "./...")
	cmd.Dir = projectRoot
	if _, err := cmd.CombinedOutput(); err != nil {
		// Don't fail, just log
		t.Logf("Test command returned error (may be expected): %v", err)
	}

	// Generate HTML
	cmd = exec.Command("go", "tool", "cover", "-html="+coverageFile, "-o", htmlFile)
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to generate HTML coverage: %v", err)
	}

	t.Logf("HTML coverage report generated: %s", htmlFile)
	t.Logf("Open in browser: file://%s", htmlFile)

	// Clean up coverage.out but keep coverage.html
	os.Remove(coverageFile)
}
