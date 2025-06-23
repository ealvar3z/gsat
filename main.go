package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"gobytes.dev/swayipc"
)

func main() {
	combo := flag.String("combo", "Mod1+Tab", "key combo for alt+tab")
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
	curFocus, _ := findFocused(tree)
	var lastFocus int64
	mu := sync.Mutex{}

	// write pid
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(fmt.Sprint(pid)), 0644); err != nil {
		log.Fatalf("write pidfile: %v", err)
	}

	// bind
	cmd := fmt.Sprintf("bindsym %s exec pkill -USR1 -F %s", *combo, pidFile)
	conn.RunCommand(cmd)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range sigs {
			switch s {
			case syscall.SIGUSR1:
				mu.Lock()
				lf := lastFocus
				mu.Unlock()
				conn.RunCommand(fmt.Sprintf("[con_id=%d] focus", lf))
			case syscall.SIGINT, syscall.SIGTERM:
				cleanup(conn, *combo, pidFile)
				os.Exit(0)
			}
		}
	}()

	if _, err := conn.Subscribe(swayipc.WindowEventType); err != nil {
		log.Fatalf("subscription to window event failed: %v", err)
	}

	conn.RegisterEventHandler(swayipc.HandlerFunc(func(e swayipc.Event) {
		handler(e, &mu, &lastFocus, &curFocus)
	}))

	// block indefinitely
	select {}
}

// handler process sway events, focusing back & forth on the last window
func handler(e swayipc.Event, mu *sync.Mutex, lastFocus, curFocus *int64) {
	if e.EventType() != swayipc.WindowEventType {
		return
	}
	we, ok := e.(*swayipc.WindowEvent)
	if !ok || we.Change != "focus" {
		return
	}
	mu.Lock()
	*lastFocus = *curFocus
	mu.Unlock()
	*curFocus = int64(we.Container.Id)
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
