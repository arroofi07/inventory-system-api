package service

import "testing"

func TestBolehSeedDemo(t *testing.T) {
	if err := BolehSeedDemo("development"); err != nil {
		t.Fatalf("development: %v", err)
	}
	if err := BolehSeedDemo("staging"); err != nil {
		t.Fatalf("staging: %v", err)
	}
	if err := BolehSeedDemo("production"); err != ErrDemoProduction {
		t.Fatalf("production want ErrDemoProduction got %v", err)
	}
	if err := BolehSeedDemo("Production"); err != ErrDemoProduction {
		t.Fatalf("Production want ErrDemoProduction got %v", err)
	}
}
