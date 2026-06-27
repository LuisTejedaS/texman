package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/luisalfredotejeda/texman/internal/collection"
	"github.com/luisalfredotejeda/texman/internal/model"
)

func main() {
	cols, paths, err := collection.LoadAll("./collections")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load collections: %v\n", err)
		cols = nil
		paths = nil
	}

	m := model.New(cols, paths, "./collections", "./responses")

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatalf("texman: %v", err)
	}
}
