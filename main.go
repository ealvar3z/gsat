package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"gobytes.dev/swayipc"
)

type focusState struct {
	mu        sync.Mutex
	last, cur int64
}

func main() {
	combo := flag.String("c", "Mod1+Tab", "key combo for alt+tab")
	flag.Parse()

	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	pidFile := filepath.Join(dir, "sway-alttab.pid")

	ctx := context.Background()
	conn, err := swayipc.Connect(ctx)
	if err != nil {
		log.Fatalf("connect %v", err)
	}

	// initial focus
	tree, err := conn.GetTree()
	if err != nil {
		log.Fatalf("get tree %v", err)
	}
	curID, _ := findFocused(tree)
	fs := &focusState{cur: curID, last: curID}

	// write pid
	pid := os.Getpid()
	pidBytes := strconv.AppendInt(nil, int64(pid), 10)
	if err := os.WriteFile(pidFile, pidBytes, 0644); err != nil {
		log.Fatalf("write pidfile: %v", err)
	}

	// bind
	bind := "bindsym " + *combo + " exec pkill -USR1 -F " + pidFile
	if _, err := conn.RunCommand(bind); err != nil {
		log.Fatalf("failed to bind key: %v", err)
	}

	if _, err := conn.Subscribe(swayipc.WindowEventType); err != nil {
		log.Fatalf("subscription to window event failed: %v", err)
	}

	conn.RegisterEventHandler(swayipc.HandlerFunc(func(e swayipc.Event) {
		handleEvent(e, fs)
	}))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range sigs {
			switch s {
			case syscall.SIGUSR1:
				fs.mu.Lock()
				target := fs.last
				fs.mu.Unlock()
				cmd := "[con_id]" + strconv.FormatInt(target, 10) + "] focus"
				if _, err := conn.RunCommand(cmd); err != nil {
					log.Printf("focus command failed: %v", err)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				cleanup(conn, *combo, pidFile)
				os.Exit(0)
			}
		}
	}()

	// block indefinitely
	select {}
}

// handleEvent process sway events, focusing back & forth on the last window
func handleEvent(e swayipc.Event, fs *focusState) {
	if e.EventType() != swayipc.WindowEventType {
		return
	}
	we, ok := e.(*swayipc.WindowEvent)
	if !ok || we.Change != "focus" {
		return
	}
	fs.mu.Lock()
	fs.last = fs.cur
	fs.mu.Unlock()
	fs.cur = int64(we.Container.Id)
}

// cleanup unbinds the key and removes the pid file
func cleanup(conn *swayipc.Conn, combo, pidFile string) {
	unbindCmd := fmt.Sprintf("unbindsym %s pkill -USR1 -F %s", combo, pidFile)
	if _, err := conn.RunCommand(unbindCmd); err != nil {
		log.Printf("failed to unbind key: %v", err)
	}
	if err := os.Remove(pidFile); err != nil {
		log.Printf("failed to remove key: %v", err)
	}
}

// findFocused traverses the tree and locates the curFocus node
func findFocused(n swayipc.Node) (int64, bool) {
	if n.Focused {
		return int64(n.Id), true
	}
	for _, child := range n.Nodes {
		if id, ok := findFocused(child); ok {
			return id, true
		}
	}
	return 0, false
}
