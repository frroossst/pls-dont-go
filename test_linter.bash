#!/usr/bin/env bash

set -e

echo "Creating test project in lint_test_go/"

# Clean up
if [ -d "lint_test_go" ]; then
    rm -rf lint_test_go
fi

mkdir -p lint_test_go
cd lint_test_go

go mod init example.com/testproject

# Create test files
cat > main.go << 'EOF'
package main

import "fmt"

// @immutable
type Config struct {
	Host     string
	Port     int
	Username string
}

// @immutable
type Point struct {
	X, Y int
}

func main() {
	cfg := Config{Host: "localhost", Port: 8080, Username: "admin"}
	cfg.Port = 9090 // should catch
	
	fmt.Println(cfg.Host)
	
	ptr := &cfg
	ptr.Username = "root" // should catch
	
	p := Point{X: 10, Y: 20}
	p.X = 15 // should catch
	
	ptr2 := &p
	_ = ptr2
	
	fmt.Println("Done")
}
EOF

cat > helper.go << 'EOF'
package main

// @immutable
type Settings struct {
	Debug   bool
	Timeout int
}

func GetSettings() Settings {
	return Settings{Debug: true, Timeout: 30}
}

func MutateSettings(s *Settings) {
	s.Debug = false // should catch
}

func ReadSettings(s *Settings) bool {
	return s.Debug
}
EOF

cd ..

echo ""
echo "Test files created!"
echo ""
echo "Now run:"
echo "  cd lint_test_go"
echo "  ../pls-dont-go ./..."
echo ""
echo "Expected: 4 errors (main.go lines 20, 24, 28 and helper.go line 14)"
echo ""


