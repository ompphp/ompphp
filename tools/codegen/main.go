package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	api := flag.String("api", "third_party/openmp-capi/apidocs/api.json", "open.mp C API metadata")
	events := flag.String("events", "third_party/openmp-capi/apidocs/events.json", "open.mp event metadata")
	constants := flag.String("constants", "tools/codegen/data/gamemode_constants.json", "curated open.mp gamemode constants")
	phpOut := flag.String("php-out", "sdk/src/Internal/api_generated.php", "generated low-level PHP API")
	publicPHPOut := flag.String("public-php-out", "sdk/src/Api", "generated public PHP API directory")
	eventOut := flag.String("event-out", "sdk/src/Event/Events.php", "generated PHP event names")
	eventHandlersOut := flag.String("event-handlers-out", "sdk/src/Event/Handlers.php", "generated typed PHP event registration methods")
	constantsOut := flag.String("constants-out", "sdk/src/Constant", "generated PHP constants directory")
	nativeHeader := flag.String("native-header", "cmd/component/native_generated.h", "generated C native adapters")
	nativeGo := flag.String("native-go", "cmd/component/native_generated.go", "generated Go native dispatch")
	eventHeader := flag.String("event-header", "cmd/component/events_generated.h", "generated C event adapters")
	eventGo := flag.String("event-go", "cmd/component/events_generated.go", "generated Go event dispatch")
	flag.Parse()
	m, err := load(*api, *events)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := generatePHP(*phpOut, m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	publicCount, err := generatePublicAPI(*publicPHPOut, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := generateEvents(*eventOut, m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := generateEventHandlers(*eventHandlersOut, m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	constantsManifest, err := loadConstants(*constants)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := generateConstants(*constantsOut, constantsManifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	count, err := generateNative(*nativeHeader, *nativeGo, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("loaded %d functions and %d events\n", len(m.Functions), len(m.Events))
	fmt.Printf("generated %d public PHP methods\n", publicCount)
	fmt.Printf("generated %d constant groups\n", len(constantsManifest.Groups))
	fmt.Printf("generated %d direct native adapters\n", count)
	if count != len(m.Functions) {
		fromID := map[string]bool{}
		for _, function := range m.Functions {
			if function.Name == function.Group+"_FromID" && ((len(function.Parameters) == 1 && isIntegerC(strings.TrimSpace(function.Parameters[0].Type))) || isCompositeEntity(function.Group)) {
				fromID[function.Group] = true
			}
		}
		reasons := map[string]int{}
		reasonNames := map[string][]string{}
		unknownEntities := map[string]int{}
		for _, function := range m.Functions {
			reason := nativeUnsupportedReason(function, fromID)
			if reason != "supported" {
				reasons[reason]++
				reasonNames[reason] = append(reasonNames[reason], function.Name)
			}
			if reason == "unknown entity input" {
				for i, p := range function.Parameters {
					if strings.TrimSpace(p.Type) == "void*" && nativeParamEntity(function, p, i) == "" {
						unknownEntities[p.Name]++
					}
				}
			}
		}
		fmt.Printf("unsupported native categories: %v\n", reasons)
		for reason, names := range reasonNames {
			if len(names) <= 30 {
				fmt.Printf("unsupported %s: %s\n", reason, strings.Join(names, ", "))
			}
		}
		if len(unknownEntities) > 0 {
			fmt.Printf("unknown entity parameters: %v\n", unknownEntities)
		}
	}
	eventCount, err := generateEventBridge(*eventHeader, *eventGo, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("generated %d event adapters\n", eventCount)
	if eventCount != len(m.Events) {
		unsupported := make([]string, 0, len(m.Events)-eventCount)
		for _, event := range m.Events {
			if !directEvent(event) {
				unsupported = append(unsupported, event.Name)
			}
		}
		fmt.Printf("unsupported events: %s\n", strings.Join(unsupported, ", "))
	}
}
