package parser

import (
	"testing"

	"github.com/example/contextiq/internal/model"
)

func TestGoParser(t *testing.T) {
	src := `package testpkg

import (
	"fmt"
	"math"
)

type Config struct {
	Port int
	Host string
}

type Service interface {
	Start() error
}

func Calculate(x float64) float64 {
	return math.Sqrt(x)
}

func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
`
	p := NewGoParser()
	node, err := p.Parse("main.go", []byte(src))
	if err != nil {
		t.Fatalf("Go parser failed: %v", err)
	}

	if node.SHA256 == "" {
		t.Error("expected non-empty SHA256")
	}

	if len(node.Imports) != 2 || node.Imports[0] != "fmt" || node.Imports[1] != "math" {
		t.Errorf("unexpected imports: %v", node.Imports)
	}

	// Verify Struct
	structID := "main.go:Config"
	structSym, ok := node.Symbols[structID]
	if !ok {
		t.Fatalf("struct symbol Config not found")
	}
	if structSym.Type != model.SymbolStruct {
		t.Errorf("expected type Struct, got %s", structSym.Type)
	}

	// Verify Interface
	interfaceID := "main.go:Service"
	interfaceSym, ok := node.Symbols[interfaceID]
	if !ok {
		t.Fatalf("interface symbol Service not found")
	}
	if interfaceSym.Type != model.SymbolInterface {
		t.Errorf("expected type Interface, got %s", interfaceSym.Type)
	}

	// Verify Function
	funcID := "main.go:Calculate"
	funcSym, ok := node.Symbols[funcID]
	if !ok {
		t.Fatalf("function symbol Calculate not found")
	}
	if funcSym.Type != model.SymbolFunction {
		t.Errorf("expected type Function, got %s", funcSym.Type)
	}

	// Verify Method
	methodID := "main.go:Config.GetAddr"
	methodSym, ok := node.Symbols[methodID]
	if !ok {
		t.Fatalf("method symbol GetAddr not found")
	}
	if methodSym.Type != model.SymbolMethod {
		t.Errorf("expected type Method, got %s", methodSym.Type)
	}
}

func TestGeneralParser_Python(t *testing.T) {
	src := `import os
import sys
from datetime import datetime

class Calculator:
    def add(self, a, b):
        # Adds two numbers
        return a + b

    def subtract(self, a, b):
        return a - b

def greet(name):
    print(f"Hello, {name}")
`
	p := NewGeneralParser()
	node, err := p.Parse("calc.py", []byte(src))
	if err != nil {
		t.Fatalf("Python parser failed: %v", err)
	}

	if len(node.Imports) < 2 {
		t.Errorf("expected imports, got: %v", node.Imports)
	}

	classID := "calc.py:Calculator"
	classSym, ok := node.Symbols[classID]
	if !ok {
		t.Fatalf("Python class Calculator not found")
	}
	if classSym.Type != model.SymbolClass {
		t.Errorf("expected class type, got %s", classSym.Type)
	}

	methodID := "calc.py:add"
	methodSym, ok := node.Symbols[methodID]
	if !ok {
		t.Fatalf("Python method add not found")
	}
	if methodSym.LineStart != 6 || methodSym.LineEnd != 9 {
		t.Errorf("expected add start at line 6, end at 9, got start=%d, end=%d", methodSym.LineStart, methodSym.LineEnd)
	}

	funcID := "calc.py:greet"
	funcSym, ok := node.Symbols[funcID]
	if !ok {
		t.Fatalf("Python function greet not found")
	}
	if funcSym.LineStart != 13 || funcSym.LineEnd != 15 {
		t.Errorf("expected greet start at line 13, end at 15, got start=%d, end=%d", funcSym.LineStart, funcSym.LineEnd)
	}
}

func TestGeneralParser_Java(t *testing.T) {
	src := `package com.example;

import java.util.List;
import java.util.ArrayList;

public class Manager {
    private List<String> list = new ArrayList<>();

    public void addName(String name) {
        if (name != null) {
            list.add(name);
        }
    }

    public List<String> getNames() {
        return list;
    }
}
`
	p := NewGeneralParser()
	node, err := p.Parse("Manager.java", []byte(src))
	if err != nil {
		t.Fatalf("Java parser failed: %v", err)
	}

	classID := "Manager.java:Manager"
	classSym, ok := node.Symbols[classID]
	if !ok {
		t.Fatalf("Java class Manager not found")
	}
	if classSym.Type != model.SymbolClass {
		t.Errorf("expected class type, got %s", classSym.Type)
	}

	methodID := "Manager.java:addName"
	methodSym, ok := node.Symbols[methodID]
	if !ok {
		t.Fatalf("Java method addName not found")
	}
	if methodSym.LineStart != 9 || methodSym.LineEnd != 13 {
		t.Errorf("expected addName start=9, end=13, got start=%d, end=%d", methodSym.LineStart, methodSym.LineEnd)
	}
}
