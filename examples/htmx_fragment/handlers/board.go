package handlers

import (
	"strconv"
	"strings"
	"sync"
)

// board is the example's data, held in memory so the sample runs without a
// database. Task is the type generated from tasks.pw.html: the rows a template
// renders are the rows this store keeps, so nothing converts between them.
type board struct {
	mu     sync.Mutex
	nextID int
	tasks  []Task
}

var tasks = &board{
	nextID: 4,
	tasks: []Task{
		{Id: "1", Title: "Draft the release notes", Owner: "ada", Priority: "high"},
		{Id: "2", Title: "Review the fragment guide", Owner: "grace", Priority: "normal"},
		{Id: "3", Title: "Archive last quarter's board", Owner: "ada", Priority: "low"},
	},
}

// list returns the tasks matching query, which is empty for an unfiltered view.
func (b *board) list(query string) []Task {
	b.mu.Lock()
	defer b.mu.Unlock()
	needle := strings.ToLower(strings.TrimSpace(query))
	matched := make([]Task, 0, len(b.tasks))
	for _, task := range b.tasks {
		if needle == "" ||
			strings.Contains(strings.ToLower(task.Title), needle) ||
			strings.Contains(strings.ToLower(task.Owner), needle) {
			matched = append(matched, task)
		}
	}
	return matched
}

func (b *board) add(title, owner, priority string) Task {
	b.mu.Lock()
	defer b.mu.Unlock()
	task := Task{Id: strconv.Itoa(b.nextID), Title: title, Owner: owner, Priority: priority}
	b.nextID++
	b.tasks = append(b.tasks, task)
	return task
}

// remove reports whether id was there, so the handler can answer a stale
// request with a 404 instead of pretending the deletion happened.
func (b *board) remove(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, task := range b.tasks {
		if task.Id == id {
			b.tasks = append(b.tasks[:i], b.tasks[i+1:]...)
			return true
		}
	}
	return false
}

func (b *board) counts() (total, high int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, task := range b.tasks {
		total++
		if task.Priority == "high" {
			high++
		}
	}
	return total, high
}
