package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"

	"delphics.delphi.tools/internal/baritems"
)

// barItem keeps one bar item alive: every stdin line becomes its text, clicks run --on-click.
// The item stays after stdin ends and goes away when the process exits.
func barItem(args []string) int {
	fs := flag.NewFlagSet("delphics bar item", flag.ContinueOnError)
	fs.Usage = func() {
		io.WriteString(fs.Output(), `usage: delphics bar item [flags] ID

Shows each line read from stdin as the text of bar item ID, until killed.
Example: while true; do date +%T; sleep 1; done | delphics bar item clock

flags:
`)
		fs.PrintDefaults()
	}
	tooltip := fs.String("tooltip", "", "tooltip `text`")
	icon := fs.String("icon", "", "icon theme `name` shown left of the text, e.g. network-vpn-symbolic")
	order := fs.Int("order", 0, "position among plugin items, lower is further left")
	bold := fs.Bool("bold", false, "bright bold text")
	onClick := fs.String("on-click", "", "shell `command` to run on click; $DELPHICS_BUTTON is 1, 2 or 3")
	urgentPrefix := fs.String("urgent-prefix", "", "lines starting with `prefix` are shown highlighted, without the prefix")
	if err := fs.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("session bus: %v", err)
		return 1
	}
	defer conn.Close()
	item, err := baritems.NewClient(conn, fs.Arg(0))
	if err != nil {
		log.Printf("bar: %v", err)
		return 1
	}
	if err := item.Set(baritems.Props{"icon": *icon, "tooltip": *tooltip, "order": int32(*order), "bold": *bold}); err != nil {
		log.Printf("bar: %v", err)
	}

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			lines <- strings.TrimRight(sc.Text(), "\r")
		}
	}()
	stop := stopSignals()
	for {
		select {
		case line := <-lines:
			urgent := false
			if *urgentPrefix != "" {
				line, urgent = strings.CutPrefix(line, *urgentPrefix)
			}
			if err := item.Set(baritems.Props{"text": line, "urgent": urgent}); err != nil {
				log.Printf("bar: %v", err)
			}
		case button := <-item.Clicks:
			if *onClick == "" {
				continue
			}
			cmd := exec.Command("sh", "-c", *onClick)
			cmd.Env = append(os.Environ(), fmt.Sprintf("DELPHICS_BUTTON=%d", button))
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			if err := cmd.Start(); err != nil {
				log.Printf("on-click: %v", err)
			} else {
				go cmd.Wait()
			}
		case <-stop:
			return 0
		}
	}
}
