package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

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
	order := fs.Int("order", 0, "position among plugin items, lower is further left")
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
	id := fs.Arg(0)

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("session bus: %v", err)
		return 1
	}
	defer conn.Close()

	props := map[string]dbus.Variant{
		"text":    dbus.MakeVariant(""),
		"tooltip": dbus.MakeVariant(*tooltip),
		"order":   dbus.MakeVariant(int32(*order)),
		"urgent":  dbus.MakeVariant(false),
	}
	set := func() {
		err := conn.Object(baritems.BusName, baritems.Path).Call(baritems.Iface+".Set", 0, id, props).Err
		if err != nil {
			log.Printf("bar: %v", err)
		}
	}

	if err := conn.AddMatchSignal(dbus.WithMatchInterface(baritems.Iface), dbus.WithMatchMember("Clicked")); err != nil {
		log.Printf("bar: %v", err)
		return 1
	}
	// a restarted bar starts empty; send the item again when it comes back
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, baritems.BusName)); err != nil {
		log.Printf("bar: %v", err)
		return 1
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			lines <- strings.TrimRight(sc.Text(), "\r")
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	set()
	for {
		select {
		case line := <-lines:
			urgent := false
			if *urgentPrefix != "" {
				line, urgent = strings.CutPrefix(line, *urgentPrefix)
			}
			props["text"], props["urgent"] = dbus.MakeVariant(line), dbus.MakeVariant(urgent)
			set()
		case sig := <-signals:
			switch {
			case sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3 && sig.Body[2] != "":
				set()
			case sig.Name == baritems.Iface+".Clicked" && len(sig.Body) == 2 && sig.Body[0] == id && *onClick != "":
				button, _ := sig.Body[1].(uint32)
				cmd := exec.Command("sh", "-c", *onClick)
				cmd.Env = append(os.Environ(), fmt.Sprintf("DELPHICS_BUTTON=%d", button))
				cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
				if err := cmd.Start(); err != nil {
					log.Printf("on-click: %v", err)
				} else {
					go cmd.Wait()
				}
			}
		case <-stop:
			return 0
		}
	}
}
