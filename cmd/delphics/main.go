// delphics is the DELPHICS command line tool.
package main

import (
	"fmt"
	"os"
)

const usage = `usage: delphics <command>

commands:
  bar item [flags] ID   show stdin lines as a bar item; run "delphics bar item -h" for flags
  brightness up|down|N  change the backlight and show it in the bar
  plugin NAME           run a built-in bar plugin: clock, network, battery,
                        volume, brightness, bluetooth, wwan
`

func main() {
	args := os.Args[1:]
	switch {
	case len(args) >= 2 && args[0] == "bar" && args[1] == "item":
		os.Exit(barItem(args[2:]))
	case len(args) >= 1 && args[0] == "brightness":
		os.Exit(brightnessCmd(args[1:]))
	case len(args) >= 1 && args[0] == "plugin":
		os.Exit(plugin(args[1:]))
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
