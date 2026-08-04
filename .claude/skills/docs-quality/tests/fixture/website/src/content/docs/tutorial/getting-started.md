---
title: 1. Getting started
description: Create the project, run it, and change the page the scaffold wrote.
sidebar:
  order: 1
  badge: advanced
---

The reader declined Tailwind at `pw init`, so nothing below may show a utility class.

```html
// handlers/home.pw.html
package handlers

export component Home(name: string): html {
  <h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

```go
// handlers/home_handler.go
package handlers

type homeInput struct {
	Name string `query:"name" default:"World"`
}

func home(w http.ResponseWriter, r *http.Request) {}
```
