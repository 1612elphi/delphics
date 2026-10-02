// delphics is the DELPHICS command line tool.
package main

import (
	"fmt"
	"os"
)

const usage = `usage: delphics <command>

commands:
  bar item [flags] ID   show stdin lines as a bar item; run "delphics bar item -h" for flags
`

func main() {
	args := os.Args[1:]
	switch {
	case len(args) >= 2 && args[0] == "bar" && args[1] == "item":
		os.Exit(barItem(args[2:]))
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
