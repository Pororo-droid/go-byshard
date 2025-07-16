package config

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	fmt.Println(GetConfig())
}
