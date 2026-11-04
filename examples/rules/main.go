// Runs entirely offline with explicitly fictional input.
package main

import (
	"fmt"
	fxmdgovaluate "github.com/fxmacrodata/govaluate-fxmacrodata"
	"log"
)

func main() {
	fixture := []byte(`{"currency":"USD","data":[],"data_quality":{"fixture":true}}`)
	result, err := fxmdgovaluate.EvaluateJSON(fixture, `field('/currency') == 'USD' && length('/data') == 0`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fictional offline fixture matched: %v\n", result.Value)
}
