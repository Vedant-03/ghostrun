package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func getTestdataPath() string {
	// Walk up to find the project root
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "basic")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func TestLoadPackage(t *testing.T) {
	testdata := getTestdataPath()
	if testdata == "" {
		t.Skip("testdata not found")
	}

	// Change to testdata dir so go/packages can find the module
	origDir, _ := os.Getwd()
	os.Chdir(testdata)
	defer os.Chdir(origDir)

	pd, err := Load("./...")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(pd.Packages) == 0 {
		t.Fatal("no packages loaded")
	}

	if len(pd.Funcs) == 0 {
		t.Fatal("no functions registered")
	}

	// Should find ProcessOrder
	funcs := pd.ListFunctions()
	t.Logf("Found functions: %v", funcs)
	if len(funcs) == 0 {
		t.Error("ListFunctions() returned empty")
	}
}

func TestFindFunction(t *testing.T) {
	testdata := getTestdataPath()
	if testdata == "" {
		t.Skip("testdata not found")
	}

	origDir, _ := os.Getwd()
	os.Chdir(testdata)
	defer os.Chdir(origDir)

	pd, err := Load("./...")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should find ProcessOrder by short name
	info, err := pd.FindFunction("ProcessOrder")
	if err != nil {
		t.Fatalf("FindFunction(ProcessOrder) error: %v", err)
	}
	if info.Name != "ProcessOrder" {
		t.Errorf("Name = %q, want %q", info.Name, "ProcessOrder")
	}
	if info.Recv != "" {
		t.Errorf("Recv should be empty for non-method, got %q", info.Recv)
	}
}

func TestFindMethod(t *testing.T) {
	testdata := getTestdataPath()
	if testdata == "" {
		t.Skip("testdata not found")
	}

	origDir, _ := os.Getwd()
	os.Chdir(testdata)
	defer os.Chdir(origDir)

	pd, err := Load("./...")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should find PercentDiscount.Calculate
	info, err := pd.FindMethod("PercentDiscount", "Calculate")
	if err != nil {
		t.Fatalf("FindMethod error: %v", err)
	}
	if info.Name != "Calculate" {
		t.Errorf("Name = %q, want %q", info.Name, "Calculate")
	}
	if info.Recv != "PercentDiscount" {
		t.Errorf("Recv = %q, want %q", info.Recv, "PercentDiscount")
	}
}
