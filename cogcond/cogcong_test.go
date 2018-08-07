package main

import (
	"testing"
	"encoding/json"
	"fmt"
	"strings"
)

func TestUnmarshalAll(t *testing.T) {
	var settings Settings
	err := json.Unmarshal([]byte(`{"all": true}`), &settings)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+v\n", settings)
}

func TestSplit(t *testing.T) {
	fmt.Printf("%v\n", strings.Split("hello", "@"))
}