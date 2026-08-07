package queries

// The four statements the application runs. Each becomes a Go function whose
// arguments and result type come from the statement itself, so a column
// renamed in a migration stops the build rather than a request.

type Todo {
  id: int
  title: string
  done: bool
}

export statement ListTodos(): sql.many<Todo> {
SELECT id, title, done FROM todos ORDER BY id
}

export statement CreateTodo(title: string): sql.exec {
INSERT INTO todos (title) VALUES ({title})
}

export statement ToggleTodo(id: int): sql.exec {
UPDATE todos SET done = NOT done WHERE id = {id}
}

export statement DeleteTodo(id: int): sql.exec {
DELETE FROM todos WHERE id = {id}
}
