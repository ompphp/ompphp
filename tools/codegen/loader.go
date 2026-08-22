package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ompphp/ompphp/tools/codegen/model"
)

type rawItem struct {
	Name       string         `json:"name"`
	ReturnType string         `json:"ret"`
	BadReturn  string         `json:"badret"`
	Parameters []rawParameter `json:"params"`
	Arguments  []rawParameter `json:"args"`
}

type rawParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func load(apiPath, eventsPath string) (model.Model, error) {
	var result model.Model
	if err := loadItems(apiPath, false, func(group string, item rawItem) {
		if group == "ComponentInterop" || group == "Network" {
			return
		}
		result.Functions = append(result.Functions, model.Function{Group: group, Name: item.Name, ReturnType: item.ReturnType, Parameters: parameters(item.Parameters)})
	}); err != nil {
		return result, err
	}
	if err := loadItems(eventsPath, true, func(group string, item rawItem) {
		result.Events = append(result.Events, model.Event{Group: group, Name: item.Name, ReturnType: item.BadReturn, Parameters: parameters(item.Arguments)})
	}); err != nil {
		return result, err
	}
	sort.Slice(result.Functions, func(i, j int) bool { return result.Functions[i].Name < result.Functions[j].Name })
	sort.Slice(result.Events, func(i, j int) bool { return result.Events[i].Name < result.Events[j].Name })
	return result, nil
}

func loadItems(path string, events bool, add func(string, rawItem)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var groups map[string][]rawItem
	if err := json.Unmarshal(data, &groups); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	for group, items := range groups {
		for _, item := range items {
			add(group, item)
		}
	}
	return nil
}

func parameters(raw []rawParameter) []model.Parameter {
	result := make([]model.Parameter, len(raw))
	for i, p := range raw {
		t := strings.TrimSpace(p.Type)
		result[i] = model.Parameter{Name: p.Name, Type: p.Type, Output: strings.HasSuffix(t, "*") && t != "void*" && t != "const char*"}
	}
	return result
}
