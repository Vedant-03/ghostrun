package scope

import (
	"testing"
	"ghostrun/internal/value"
)

func TestDefineAndGet(t *testing.T) {
	s := New("test")
	s.Define("x", value.NewInt(42))

	v, ok := s.Get("x")
	if !ok {
		t.Fatal("Get(x) returned false")
	}
	if v.GoValue() != int64(42) {
		t.Errorf("Get(x) = %v, want 42", v.GoValue())
	}

	_, ok = s.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}
}

func TestSet(t *testing.T) {
	s := New("test")
	s.Define("x", value.NewInt(1))

	err := s.Set("x", value.NewInt(2))
	if err != nil {
		t.Fatalf("Set(x) error: %v", err)
	}

	v, _ := s.Get("x")
	if v.GoValue() != int64(2) {
		t.Errorf("After Set, x = %v, want 2", v.GoValue())
	}

	err = s.Set("missing", value.NewInt(0))
	if err == nil {
		t.Error("Set(missing) should return error")
	}
}

func TestChildScope(t *testing.T) {
	parent := New("parent")
	parent.Define("x", value.NewInt(1))
	parent.Define("y", value.NewInt(2))

	child := parent.Child("child")
	child.Define("y", value.NewInt(20)) // shadow parent's y
	child.Define("z", value.NewInt(30))

	// Child sees parent's x
	v, ok := child.Get("x")
	if !ok || v.GoValue() != int64(1) {
		t.Errorf("child.Get(x) = %v, %v; want 1, true", v, ok)
	}

	// Child sees its own y (shadow)
	v, ok = child.Get("y")
	if !ok || v.GoValue() != int64(20) {
		t.Errorf("child.Get(y) = %v, %v; want 20, true", v, ok)
	}

	// Child sees its own z
	v, ok = child.Get("z")
	if !ok || v.GoValue() != int64(30) {
		t.Errorf("child.Get(z) = %v, %v; want 30, true", v, ok)
	}

	// Parent doesn't see child's z
	_, ok = parent.Get("z")
	if ok {
		t.Error("parent.Get(z) should return false")
	}

	// Parent's y is unchanged
	v, _ = parent.Get("y")
	if v.GoValue() != int64(2) {
		t.Errorf("parent.Get(y) = %v, want 2", v.GoValue())
	}
}

func TestSetInParent(t *testing.T) {
	parent := New("parent")
	parent.Define("x", value.NewInt(1))

	child := parent.Child("child")

	// Set x through child should update parent's x
	err := child.Set("x", value.NewInt(99))
	if err != nil {
		t.Fatalf("child.Set(x) error: %v", err)
	}

	// Parent sees the update
	v, _ := parent.Get("x")
	if v.GoValue() != int64(99) {
		t.Errorf("parent.Get(x) = %v, want 99", v.GoValue())
	}
}

func TestSnapshot(t *testing.T) {
	parent := New("parent")
	parent.Define("x", value.NewInt(1))
	parent.Define("y", value.NewInt(2))

	child := parent.Child("child")
	child.Define("y", value.NewInt(20))
	child.Define("z", value.NewInt(30))

	snap := child.Snapshot()

	if len(snap) != 3 {
		t.Errorf("Snapshot has %d vars, want 3", len(snap))
	}
	if snap["x"].GoValue() != int64(1) {
		t.Errorf("snap[x] = %v, want 1", snap["x"].GoValue())
	}
	if snap["y"].GoValue() != int64(20) {
		t.Errorf("snap[y] = %v, want 20 (shadowed)", snap["y"].GoValue())
	}
	if snap["z"].GoValue() != int64(30) {
		t.Errorf("snap[z] = %v, want 30", snap["z"].GoValue())
	}
}

func TestLocalSnapshot(t *testing.T) {
	parent := New("parent")
	parent.Define("x", value.NewInt(1))

	child := parent.Child("child")
	child.Define("y", value.NewInt(2))

	local := child.LocalSnapshot()
	if len(local) != 1 {
		t.Errorf("LocalSnapshot has %d vars, want 1", len(local))
	}
	if _, ok := local["x"]; ok {
		t.Error("LocalSnapshot should not include parent vars")
	}
}

func TestDepth(t *testing.T) {
	root := New("root")
	if root.Depth() != 0 {
		t.Errorf("root.Depth() = %d, want 0", root.Depth())
	}

	child := root.Child("child")
	if child.Depth() != 1 {
		t.Errorf("child.Depth() = %d, want 1", child.Depth())
	}

	grandchild := child.Child("grandchild")
	if grandchild.Depth() != 2 {
		t.Errorf("grandchild.Depth() = %d, want 2", grandchild.Depth())
	}
}

func TestHasLocal(t *testing.T) {
	parent := New("parent")
	parent.Define("x", value.NewInt(1))

	child := parent.Child("child")

	if child.HasLocal("x") {
		t.Error("child.HasLocal(x) should be false")
	}
	if !child.Has("x") {
		t.Error("child.Has(x) should be true (from parent)")
	}
}
