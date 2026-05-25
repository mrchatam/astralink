// AstraLink transport benchmark harness (adapted from upstream bench tooling).
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	runs := flag.Int("runs", 3, "benchmark runs")
	flag.Parse()
	fmt.Printf("AstraLink bench placeholder (%d runs)\n", *runs)
	fmt.Println("Use full integration bench once server/client binaries are deployed.")
	os.Exit(0)
}
