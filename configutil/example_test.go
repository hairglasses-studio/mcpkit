//go:build !official_sdk

package configutil_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hairglasses-studio/mcpkit/configutil"
)

type AppConfig struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

func ExampleSaveJSON() {
	dir, _ := os.MkdirTemp("", "configutil-example-*")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "config.json")
	cfg := AppConfig{Name: "myapp", Port: 8080}

	if err := configutil.SaveJSON(path, cfg); err != nil {
		fmt.Println("error:", err)
		return
	}

	loaded, err := configutil.LoadJSON[AppConfig](path)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%s:%d\n", loaded.Name, loaded.Port)
	// Output:
	// myapp:8080
}
